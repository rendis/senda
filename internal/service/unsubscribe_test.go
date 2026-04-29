package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/unsubscribe"
	"github.com/rendis/senda/pkg/apperr"
)

// ---- helpers ----

func unsub_testKey(offset byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i+1) + offset
	}
	return k
}

func unsub_fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
}

func unsub_makeToken(t *testing.T, key []byte, ws uuid.UUID, slug, email string, sourceEmailID uuid.UUID, now time.Time) string {
	t.Helper()
	p := unsubscribe.Payload{
		Version:          1,
		WorkspaceID:      ws,
		TemplateTypeSlug: slug,
		TemplateTypeName: slug + "-name",
		Email:            email,
		SourceEmailID:    sourceEmailID,
		IssuedAt:         now,
		ExpiresAt:        now.Add(24 * time.Hour),
	}
	tok, err := unsubscribe.Generate(p, key)
	if err != nil {
		t.Fatalf("Generate token: %v", err)
	}
	return tok
}

// ---- fakes ----

type fakeWorkspaceLookup struct {
	ws  *domain.Workspace
	key []byte
	err error
}

func (f *fakeWorkspaceLookup) GetByID(_ context.Context, _ uuid.UUID) (*domain.Workspace, error) {
	return f.ws, f.err
}

func (f *fakeWorkspaceLookup) GetUnsubscribeSigningKey(_ context.Context, _ uuid.UUID) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.key, nil
}

type fakeTemplateTypeLookup struct {
	tt  *domain.TemplateType
	err error
}

func (f *fakeTemplateTypeLookup) FindTypeBySlugInScope(_ context.Context, _ string, _ *uuid.UUID) (*domain.TemplateType, error) {
	return f.tt, f.err
}

type fakeSuppressionWS struct {
	addCalled    bool
	removeCalled bool
	removeReason string
	active       *domain.SuppressionWorkspace
	addErr       error
	getErr       error
	removeErr    error
}

func (f *fakeSuppressionWS) AddWorkspace(_ context.Context, sup *domain.SuppressionWorkspace) error {
	f.addCalled = true
	f.active = sup
	return f.addErr
}

func (f *fakeSuppressionWS) GetActiveWorkspaceSuppression(_ context.Context, _ uuid.UUID, _ string) (*domain.SuppressionWorkspace, error) {
	return f.active, f.getErr
}

func (f *fakeSuppressionWS) RemoveWorkspaceSuppression(_ context.Context, _ uuid.UUID, _ string, reason string) error {
	f.removeCalled = true
	f.removeReason = reason
	return f.removeErr
}

type fakeTTSWriter struct {
	upserted []*domain.TemplateTypeSubscription
	state    *domain.TemplateTypeSubscription
	stateErr error
	listRows []*domain.TemplateTypeSubscription
}

func (f *fakeTTSWriter) Upsert(_ context.Context, sub *domain.TemplateTypeSubscription) error {
	f.upserted = append(f.upserted, sub)
	return nil
}

func (f *fakeTTSWriter) GetState(_ context.Context, _, _ uuid.UUID, _ string) (*domain.TemplateTypeSubscription, error) {
	return f.state, f.stateErr
}

func (f *fakeTTSWriter) ListOptOutsForRecipient(_ context.Context, _ uuid.UUID, _ string) ([]*domain.TemplateTypeSubscription, error) {
	return f.listRows, nil
}

type fakeEmailHistory struct {
	types []EmailHistoryType
	err   error
}

func (f *fakeEmailHistory) DistinctTemplateTypesForRecipient(_ context.Context, _ uuid.UUID, _ string, _ time.Time) ([]EmailHistoryType, error) {
	return f.types, f.err
}

// ---- test helpers ----

func newTestService(
	wsLookup unsubWorkspaceLookup,
	ttLookup unsubTemplateTypeLookup,
	supWS unsubSuppressionWS,
	tts unsubTTSWriter,
	history unsubEmailHistory,
) *UnsubscribeService {
	svc := NewUnsubscribeService(wsLookup, ttLookup, supWS, tts, history)
	svc.now = unsub_fixedTime
	return svc
}

// ---- tests ----

func TestUnsubscribeService_OneClickOptOut_WritesRow(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000001")
	ttID := uuid.New()
	key := unsub_testKey(0)
	now := unsub_fixedTime()
	tok := unsub_makeToken(t, key, ws, "newsletter", "user@example.com", uuid.New(), now)

	wsLookup := &fakeWorkspaceLookup{
		key: key,
		ws:  &domain.Workspace{ID: ws, Name: "Acme"},
	}
	ttLookup := &fakeTemplateTypeLookup{
		tt: &domain.TemplateType{ID: ttID, Slug: "newsletter", Name: "Newsletter"},
	}
	supWS := &fakeSuppressionWS{getErr: apperr.NotFound("no active suppression")}
	tts := &fakeTTSWriter{}
	history := &fakeEmailHistory{}

	svc := newTestService(wsLookup, ttLookup, supWS, tts, history)
	if err := svc.OneClickOptOut(context.Background(), tok); err != nil {
		t.Fatalf("OneClickOptOut: %v", err)
	}
	if len(tts.upserted) != 1 {
		t.Fatalf("expected 1 Upsert call, got %d", len(tts.upserted))
	}
	if tts.upserted[0].Subscribed {
		t.Fatal("upserted row must have Subscribed=false")
	}
	if tts.upserted[0].Source != domain.SubscriptionSourceRecipientOptout {
		t.Fatalf("Source = %q, want recipient_optout", tts.upserted[0].Source)
	}
}

func TestUnsubscribeService_OneClickOptOut_RejectsBadKey(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000002")
	key1 := unsub_testKey(0)
	key2 := unsub_testKey(0xAA)
	now := unsub_fixedTime()
	tok := unsub_makeToken(t, key1, ws, "newsletter", "user@example.com", uuid.New(), now)

	wsLookup := &fakeWorkspaceLookup{key: key2, ws: &domain.Workspace{ID: ws, Name: "Acme"}}
	tts := &fakeTTSWriter{}
	svc := newTestService(wsLookup, &fakeTemplateTypeLookup{}, &fakeSuppressionWS{}, tts, &fakeEmailHistory{})

	err := svc.OneClickOptOut(context.Background(), tok)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
	if len(tts.upserted) != 0 {
		t.Fatal("Upsert must not be called when token is invalid")
	}
}

func TestUnsubscribeService_OneClickOptOut_RejectsMalformedToken(t *testing.T) {
	wsLookup := &fakeWorkspaceLookup{key: unsub_testKey(0), ws: &domain.Workspace{Name: "Acme"}}
	svc := newTestService(wsLookup, &fakeTemplateTypeLookup{}, &fakeSuppressionWS{}, &fakeTTSWriter{}, &fakeEmailHistory{})

	err := svc.OneClickOptOut(context.Background(), "garbage")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for malformed token, got %v", err)
	}
}

func TestUnsubscribeService_OneClickOptOut_RejectsExpiredToken(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000003")
	key := unsub_testKey(0)
	// Token issued and expired 2 hours ago; service's now = fixedTime
	past := unsub_fixedTime().Add(-2 * time.Hour)
	p := unsubscribe.Payload{
		Version:          1,
		WorkspaceID:      ws,
		TemplateTypeSlug: "newsletter",
		TemplateTypeName: "Newsletter",
		Email:            "user@example.com",
		SourceEmailID:    uuid.New(),
		IssuedAt:         past,
		ExpiresAt:        past.Add(time.Hour), // expires 1 hour ago
	}
	tok, _ := unsubscribe.Generate(p, key)

	wsLookup := &fakeWorkspaceLookup{key: key, ws: &domain.Workspace{ID: ws, Name: "Acme"}}
	svc := newTestService(wsLookup, &fakeTemplateTypeLookup{}, &fakeSuppressionWS{}, &fakeTTSWriter{}, &fakeEmailHistory{})

	err := svc.OneClickOptOut(context.Background(), tok)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestUnsubscribeService_OptOutAll_WritesWorkspaceSuppression(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000004")
	key := unsub_testKey(0)
	now := unsub_fixedTime()
	tok := unsub_makeToken(t, key, ws, "promo", "user@example.com", uuid.New(), now)

	wsLookup := &fakeWorkspaceLookup{key: key, ws: &domain.Workspace{ID: ws, Name: "Acme"}}
	supWS := &fakeSuppressionWS{}
	svc := newTestService(wsLookup, &fakeTemplateTypeLookup{}, supWS, &fakeTTSWriter{}, &fakeEmailHistory{})

	if err := svc.OptOutAll(context.Background(), tok); err != nil {
		t.Fatalf("OptOutAll: %v", err)
	}
	if !supWS.addCalled {
		t.Fatal("Add must be called on workspace suppression store")
	}
	if supWS.active.Reason != domain.SuppressionUnsubscribe {
		t.Fatalf("Reason = %q, want unsubscribe", supWS.active.Reason)
	}
}

func TestUnsubscribeService_GetContext_ReturnsCurrentState(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000005")
	ttID := uuid.New()
	key := unsub_testKey(0)
	now := unsub_fixedTime()
	tok := unsub_makeToken(t, key, ws, "newsletter", "user@example.com", uuid.New(), now)

	wsLookup := &fakeWorkspaceLookup{key: key, ws: &domain.Workspace{ID: ws, Name: "Acme Corp"}}
	ttLookup := &fakeTemplateTypeLookup{
		tt: &domain.TemplateType{ID: ttID, Slug: "newsletter", Name: "Newsletter"},
	}
	// Recipient opted out of the type
	tts := &fakeTTSWriter{
		state: &domain.TemplateTypeSubscription{Subscribed: false},
	}
	// Not opted out of all
	supWS := &fakeSuppressionWS{getErr: apperr.NotFound("none")}

	svc := newTestService(wsLookup, ttLookup, supWS, tts, &fakeEmailHistory{})
	ctx, err := svc.GetContext(context.Background(), tok)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if ctx.WorkspaceName != "Acme Corp" {
		t.Errorf("WorkspaceName = %q, want Acme Corp", ctx.WorkspaceName)
	}
	if !ctx.OptedOutOfType {
		t.Error("OptedOutOfType must be true")
	}
	if ctx.OptedOutOfAll {
		t.Error("OptedOutOfAll must be false")
	}
	if ctx.Email != "user@example.com" {
		t.Errorf("Email = %q, want user@example.com", ctx.Email)
	}
}

func TestUnsubscribeService_GetPreferences_ListsHistoricalTypesWithStates(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000006")
	key := unsub_testKey(0)
	now := unsub_fixedTime()
	tok := unsub_makeToken(t, key, ws, "newsletter", "user@example.com", uuid.New(), now)

	wsLookup := &fakeWorkspaceLookup{key: key, ws: &domain.Workspace{ID: ws, Name: "Acme"}}

	// History returns 2 slugs
	history := &fakeEmailHistory{
		types: []EmailHistoryType{
			{Slug: "newsletter", LastSentAt: now.Add(-24 * time.Hour)},
			{Slug: "promo", LastSentAt: now.Add(-48 * time.Hour)},
		},
	}

	newsID := uuid.New()
	promoID := uuid.New()

	// tt lookup returns matching type based on slug
	ttResponses := map[string]*domain.TemplateType{
		"newsletter": {ID: newsID, Slug: "newsletter", Name: "Newsletter"},
		"promo":      {ID: promoID, Slug: "promo", Name: "Promotions"},
	}
	callCount := 0
	slugOrder := []string{"newsletter", "promo"}
	ttLookup := &multiTTLookup{responses: ttResponses, order: slugOrder, idx: &callCount}

	// newsletter opted out; promo has no row (subscribed=true by default)
	subCalls := 0
	tts := &multiStateTTSWriter{
		states: map[uuid.UUID]*domain.TemplateTypeSubscription{
			newsID: {Subscribed: false},
		},
		subCalls: &subCalls,
	}

	supWS := &fakeSuppressionWS{getErr: apperr.NotFound("none")}
	svc := NewUnsubscribeService(wsLookup, ttLookup, supWS, tts, history)
	svc.now = unsub_fixedTime

	view, err := svc.GetPreferences(context.Background(), tok)
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if len(view.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(view.Entries))
	}
	// Find newsletter entry
	var newsEntry, promoEntry *PreferencesEntry
	for i := range view.Entries {
		e := &view.Entries[i]
		switch e.TemplateTypeSlug {
		case "newsletter":
			newsEntry = e
		case "promo":
			promoEntry = e
		}
	}
	if newsEntry == nil {
		t.Fatal("newsletter entry missing")
	}
	if promoEntry == nil {
		t.Fatal("promo entry missing")
	}
	if newsEntry.Subscribed {
		t.Error("newsletter must have Subscribed=false (opted out)")
	}
	if !promoEntry.Subscribed {
		t.Error("promo must have Subscribed=true (no row)")
	}
}

func TestUnsubscribeService_UpdatePreferences_UpsertsOnePerChange(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000007")
	key := unsub_testKey(0)
	now := unsub_fixedTime()
	tok := unsub_makeToken(t, key, ws, "newsletter", "user@example.com", uuid.New(), now)

	wsLookup := &fakeWorkspaceLookup{key: key, ws: &domain.Workspace{ID: ws, Name: "Acme"}}

	newsID := uuid.New()
	promoID := uuid.New()
	ttResponses := map[string]*domain.TemplateType{
		"newsletter": {ID: newsID, Slug: "newsletter", Name: "Newsletter"},
		"promo":      {ID: promoID, Slug: "promo", Name: "Promotions"},
	}
	callCount := 0
	slugOrder := []string{"newsletter", "promo"}
	ttLookup := &multiTTLookup{responses: ttResponses, order: slugOrder, idx: &callCount}

	tts := &fakeTTSWriter{}
	supWS := &fakeSuppressionWS{}
	svc := newTestService(wsLookup, ttLookup, supWS, tts, &fakeEmailHistory{})

	changes := []PreferenceChange{
		{TemplateTypeSlug: "newsletter", Subscribed: false},
		{TemplateTypeSlug: "promo", Subscribed: true},
	}
	if err := svc.UpdatePreferences(context.Background(), tok, changes); err != nil {
		t.Fatalf("UpdatePreferences: %v", err)
	}
	if len(tts.upserted) != 2 {
		t.Fatalf("expected 2 Upsert calls, got %d", len(tts.upserted))
	}
	if tts.upserted[0].Subscribed {
		t.Error("first change (newsletter): Subscribed must be false")
	}
	if !tts.upserted[1].Subscribed {
		t.Error("second change (promo): Subscribed must be true")
	}
}

func TestUnsubscribeService_Resubscribe_RemovesWorkspaceSuppression(t *testing.T) {
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000008")
	key := unsub_testKey(0)
	now := unsub_fixedTime()
	tok := unsub_makeToken(t, key, ws, "newsletter", "user@example.com", uuid.New(), now)

	wsLookup := &fakeWorkspaceLookup{key: key, ws: &domain.Workspace{ID: ws, Name: "Acme"}}
	supWS := &fakeSuppressionWS{}
	svc := newTestService(wsLookup, &fakeTemplateTypeLookup{}, supWS, &fakeTTSWriter{}, &fakeEmailHistory{})

	if err := svc.Resubscribe(context.Background(), tok); err != nil {
		t.Fatalf("Resubscribe: %v", err)
	}
	if !supWS.removeCalled {
		t.Fatal("Remove must be called on workspace suppression store")
	}
	if supWS.removeReason != "recipient_resubscribe" {
		t.Fatalf("Remove reason = %q, want recipient_resubscribe", supWS.removeReason)
	}
}

// ---- multi-type fakes for preference tests ----

// multiTTLookup returns responses keyed by slug in the order provided.
type multiTTLookup struct {
	responses map[string]*domain.TemplateType
	order     []string
	idx       *int
}

func (m *multiTTLookup) FindTypeBySlugInScope(_ context.Context, slug string, _ *uuid.UUID) (*domain.TemplateType, error) {
	tt, ok := m.responses[slug]
	if !ok {
		return nil, apperr.NotFound("type %q not found", slug)
	}
	return tt, nil
}

// multiStateTTSWriter returns subscription state keyed by templateTypeID.
type multiStateTTSWriter struct {
	states   map[uuid.UUID]*domain.TemplateTypeSubscription
	upserted []*domain.TemplateTypeSubscription
	subCalls *int
}

func (m *multiStateTTSWriter) Upsert(_ context.Context, sub *domain.TemplateTypeSubscription) error {
	m.upserted = append(m.upserted, sub)
	return nil
}

func (m *multiStateTTSWriter) GetState(_ context.Context, _, templateTypeID uuid.UUID, _ string) (*domain.TemplateTypeSubscription, error) {
	if m.subCalls != nil {
		*m.subCalls++
	}
	sub, ok := m.states[templateTypeID]
	if !ok {
		return nil, apperr.NotFound("no subscription state")
	}
	return sub, nil
}

func (m *multiStateTTSWriter) ListOptOutsForRecipient(_ context.Context, _ uuid.UUID, _ string) ([]*domain.TemplateTypeSubscription, error) {
	return nil, nil
}
