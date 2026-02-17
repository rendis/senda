package river

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	goriver "github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// --- Mock DNS resolver ---

type mockDNSResolver struct {
	lookupTXTFn func(ctx context.Context, name string) ([]string, error)
}

func (m *mockDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if m.lookupTXTFn != nil {
		return m.lookupTXTFn(ctx, name)
	}
	return nil, nil
}

// --- Mock domain store for verify tests ---

type mockDomainStoreV struct {
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Domain, error)
	updateFn  func(ctx context.Context, d *domain.Domain) error

	updateCalls []*domain.Domain
}

func (m *mockDomainStoreV) Create(ctx context.Context, d *domain.Domain) error { return nil }
func (m *mockDomainStoreV) GetByID(ctx context.Context, id uuid.UUID) (*domain.Domain, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, domain.ErrNotFound
}
func (m *mockDomainStoreV) Update(ctx context.Context, d *domain.Domain) error {
	m.updateCalls = append(m.updateCalls, d)
	if m.updateFn != nil {
		return m.updateFn(ctx, d)
	}
	return nil
}
func (m *mockDomainStoreV) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockDomainStoreV) ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error) {
	return nil, nil
}
func (m *mockDomainStoreV) ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts port.ListOptions) (*port.PageResult[domain.Domain], error) {
	return nil, nil
}
func (m *mockDomainStoreV) GetPendingVerifications(ctx context.Context, limit int) ([]*domain.Domain, error) {
	return nil, nil
}

// --- Test helpers ---

func newTestDomain() *domain.Domain {
	return &domain.Domain{
		ID:            uuid.Must(uuid.NewV7()),
		DomainName:    "example.com",
		DKIMSelector:  "senda",
		DKIMPublicKey: "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQ==",
		Status:        domain.DomainStatusPending,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
}

func makeVerifyJob(domainID uuid.UUID) *goriver.Job[VerifyJobArgs] {
	return &goriver.Job[VerifyJobArgs]{
		Args: VerifyJobArgs{DomainID: domainID},
		JobRow: &rivertype.JobRow{
			Attempt: 1,
		},
	}
}

// --- Tests ---

func TestVerifyWorker_DomainNotFound_CancelsJob(t *testing.T) {
	store := &mockDomainStoreV{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Domain, error) {
			return nil, domain.ErrNotFound
		},
	}
	worker := NewVerifyWorker(store, &mockDNSResolver{})

	job := makeVerifyJob(uuid.Must(uuid.NewV7()))
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cancelErr *goriver.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Errorf("expected JobCancelError, got %T: %v", err, err)
	}
}

func TestVerifyWorker_DNSLookupFails_SetsErrorStatus(t *testing.T) {
	d := newTestDomain()
	store := &mockDomainStoreV{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Domain, error) {
			if id == d.ID {
				return d, nil
			}
			return nil, domain.ErrNotFound
		},
	}
	dns := &mockDNSResolver{
		lookupTXTFn: func(_ context.Context, _ string) ([]string, error) {
			return nil, errors.New("dns: NXDOMAIN")
		},
	}
	worker := NewVerifyWorker(store, dns)

	job := makeVerifyJob(d.ID)
	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(store.updateCalls))
	}
	updated := store.updateCalls[0]
	if updated.Status != domain.DomainStatusError {
		t.Errorf("status = %q, want %q", updated.Status, domain.DomainStatusError)
	}
	if updated.LastError == nil {
		t.Error("expected LastError to be set")
	}
	if updated.LastCheckAt == nil {
		t.Error("expected LastCheckAt to be set")
	}
	if updated.NextCheckAt == nil {
		t.Error("expected NextCheckAt to be set")
	}
}

func TestVerifyWorker_DKIMRecordFound_Verified(t *testing.T) {
	d := newTestDomain()
	store := &mockDomainStoreV{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Domain, error) {
			return d, nil
		},
	}
	expectedTXT := "v=DKIM1; k=rsa; p=" + d.DKIMPublicKey
	dns := &mockDNSResolver{
		lookupTXTFn: func(_ context.Context, name string) ([]string, error) {
			// Return the expected DKIM record.
			return []string{expectedTXT}, nil
		},
	}
	worker := NewVerifyWorker(store, dns)

	job := makeVerifyJob(d.ID)
	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(store.updateCalls))
	}
	updated := store.updateCalls[0]
	if updated.Status != domain.DomainStatusVerified {
		t.Errorf("status = %q, want %q", updated.Status, domain.DomainStatusVerified)
	}
	if updated.VerifiedAt == nil {
		t.Error("expected VerifiedAt to be set")
	}
	if updated.LastError != nil {
		t.Errorf("expected LastError to be nil, got %q", *updated.LastError)
	}
}

func TestVerifyWorker_DKIMRecordNotFound_Error(t *testing.T) {
	d := newTestDomain()
	store := &mockDomainStoreV{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Domain, error) {
			return d, nil
		},
	}
	dns := &mockDNSResolver{
		lookupTXTFn: func(_ context.Context, name string) ([]string, error) {
			// Return records that don't match.
			return []string{"v=spf1 include:_spf.google.com ~all"}, nil
		},
	}
	worker := NewVerifyWorker(store, dns)

	job := makeVerifyJob(d.ID)
	err := worker.Work(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(store.updateCalls))
	}
	updated := store.updateCalls[0]
	if updated.Status != domain.DomainStatusError {
		t.Errorf("status = %q, want %q", updated.Status, domain.DomainStatusError)
	}
	if updated.LastError == nil {
		t.Error("expected LastError to be set")
	}
}

func TestVerifyWorker_NextCheckAt_24Hours(t *testing.T) {
	d := newTestDomain()
	store := &mockDomainStoreV{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Domain, error) {
			return d, nil
		},
	}
	dns := &mockDNSResolver{
		lookupTXTFn: func(_ context.Context, name string) ([]string, error) {
			return []string{"v=DKIM1; k=rsa; p=" + d.DKIMPublicKey}, nil
		},
	}
	worker := NewVerifyWorker(store, dns)

	before := time.Now().UTC()
	job := makeVerifyJob(d.ID)
	_ = worker.Work(context.Background(), job)

	if len(store.updateCalls) == 0 {
		t.Fatal("expected update call")
	}
	updated := store.updateCalls[0]
	if updated.NextCheckAt == nil {
		t.Fatal("expected NextCheckAt to be set")
	}
	// NextCheckAt should be ~24h from now.
	diff := updated.NextCheckAt.Sub(before)
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Errorf("NextCheckAt diff = %v, expected ~24h", diff)
	}
}

func TestVerifyWorker_UpdateFails_ReturnsError(t *testing.T) {
	d := newTestDomain()
	store := &mockDomainStoreV{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Domain, error) {
			return d, nil
		},
		updateFn: func(_ context.Context, _ *domain.Domain) error {
			return errors.New("db connection lost")
		},
	}
	dns := &mockDNSResolver{
		lookupTXTFn: func(_ context.Context, name string) ([]string, error) {
			return []string{"v=DKIM1; k=rsa; p=" + d.DKIMPublicKey}, nil
		},
	}
	worker := NewVerifyWorker(store, dns)

	job := makeVerifyJob(d.ID)
	err := worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected error from failed update, got nil")
	}
}
