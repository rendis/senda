package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	goriver "github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	riveradapter "github.com/rendis/senda/internal/adapter/river"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
	"github.com/rendis/senda/internal/service"
)

type pipelineEmailStore struct {
	hotByID        map[uuid.UUID]*domain.Email
	hotByTracking  map[string]*domain.Email
	payloadByEmail map[uuid.UUID]*domain.EmailPayload
	events         []*domain.EmailEvent

	getPayloadCalls []uuid.UUID
}

func newPipelineEmailStore() *pipelineEmailStore {
	return &pipelineEmailStore{
		hotByID:        make(map[uuid.UUID]*domain.Email),
		hotByTracking:  make(map[string]*domain.Email),
		payloadByEmail: make(map[uuid.UUID]*domain.EmailPayload),
	}
}

func (s *pipelineEmailStore) Create(_ context.Context, email *domain.Email) error {
	hot := clonePipelineEmail(email)
	payload := &domain.EmailPayload{
		EmailID:           email.ID,
		EmailCreatedAt:    email.CreatedAt,
		BodyMJML:          email.BodyMJML,
		VariablesSnapshot: clonePipelineVars(email.VariablesSnapshot),
		InjectorsSnapshot: clonePipelineInjectors(email.InjectorsSnapshot),
		CreatedAt:         email.CreatedAt,
		UpdatedAt:         email.UpdatedAt,
	}

	hot.BodyMJML = ""
	hot.VariablesSnapshot = nil
	hot.InjectorsSnapshot = nil

	s.hotByID[email.ID] = hot
	s.hotByTracking[email.TrackingID] = hot
	s.payloadByEmail[email.ID] = payload
	return nil
}

func (s *pipelineEmailStore) CreateTx(ctx context.Context, _ pgx.Tx, email *domain.Email) error {
	return s.Create(ctx, email)
}

func (s *pipelineEmailStore) GetByTrackingID(_ context.Context, trackingID string) (*domain.Email, error) {
	email, ok := s.hotByTracking[trackingID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return clonePipelineEmail(email), nil
}

func (s *pipelineEmailStore) GetByProviderMessageID(_ context.Context, providerMessageID string) (*domain.Email, error) {
	for _, email := range s.hotByID {
		if email.ProviderMessageID != nil && *email.ProviderMessageID == providerMessageID {
			return clonePipelineEmail(email), nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *pipelineEmailStore) GetPayload(_ context.Context, emailID uuid.UUID) (*domain.EmailPayload, error) {
	s.getPayloadCalls = append(s.getPayloadCalls, emailID)
	payload, ok := s.payloadByEmail[emailID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &domain.EmailPayload{
		EmailID:           payload.EmailID,
		EmailCreatedAt:    payload.EmailCreatedAt,
		BodyMJML:          payload.BodyMJML,
		VariablesSnapshot: clonePipelineVars(payload.VariablesSnapshot),
		InjectorsSnapshot: clonePipelineInjectors(payload.InjectorsSnapshot),
		CreatedAt:         payload.CreatedAt,
		UpdatedAt:         payload.UpdatedAt,
	}, nil
}

func (s *pipelineEmailStore) PurgeWorkspaceRuntime(_ context.Context, _ uuid.UUID) error { return nil }

func (s *pipelineEmailStore) UpdateStatus(_ context.Context, id uuid.UUID, newStatus, expectedStatus domain.EmailStatus) error {
	email, ok := s.hotByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if email.Status != expectedStatus {
		return domain.ErrStatusConflict
	}
	email.Status = newStatus
	email.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *pipelineEmailStore) UpdateRetry(_ context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error {
	email, ok := s.hotByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	email.RetryCount = retryCount
	email.NextRetryAt = nextRetryAt
	return nil
}

func (s *pipelineEmailStore) SetProviderMessageID(_ context.Context, id uuid.UUID, providerMessageID string) error {
	email, ok := s.hotByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	email.ProviderMessageID = &providerMessageID
	return nil
}

func (s *pipelineEmailStore) AddEvent(_ context.Context, event *domain.EmailEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *pipelineEmailStore) AddEventTx(_ context.Context, _ pgx.Tx, event *domain.EmailEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *pipelineEmailStore) GetEvents(_ context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error) {
	var events []*domain.EmailEvent
	for _, event := range s.events {
		if event.EmailID == emailID {
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *pipelineEmailStore) QueryByExternalID(_ context.Context, _ uuid.UUID, _ string, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}

func (s *pipelineEmailStore) QueryByRecipient(_ context.Context, _ uuid.UUID, _ string, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}

func (s *pipelineEmailStore) QueryByWorkspace(_ context.Context, _ uuid.UUID, _ port.EmailFilters, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}

func (s *pipelineEmailStore) QueryByExternalIDGlobal(_ context.Context, _ string, _ string, _ int) ([]*domain.Email, string, error) {
	return nil, "", nil
}
func (s *pipelineEmailStore) DistinctTemplateTypesForRecipient(_ context.Context, _ uuid.UUID, _ string, _ time.Time) ([]port.EmailHistoryType, error) {
	return nil, nil
}

func (s *pipelineEmailStore) hotByTrackingID(trackingID string) *domain.Email {
	email, ok := s.hotByTracking[trackingID]
	if !ok {
		return nil
	}
	return clonePipelineEmail(email)
}

func (s *pipelineEmailStore) payloadByID(emailID uuid.UUID) *domain.EmailPayload {
	payload, ok := s.payloadByEmail[emailID]
	if !ok {
		return nil
	}
	return &domain.EmailPayload{
		EmailID:           payload.EmailID,
		EmailCreatedAt:    payload.EmailCreatedAt,
		BodyMJML:          payload.BodyMJML,
		VariablesSnapshot: clonePipelineVars(payload.VariablesSnapshot),
		InjectorsSnapshot: clonePipelineInjectors(payload.InjectorsSnapshot),
		CreatedAt:         payload.CreatedAt,
		UpdatedAt:         payload.UpdatedAt,
	}
}

type pipelineQueue struct {
	sendJobs []*port.SendJob
}

func (q *pipelineQueue) EnqueueSend(_ context.Context, job *port.SendJob) error {
	cloned := *job
	q.sendJobs = append(q.sendJobs, &cloned)
	return nil
}

func (q *pipelineQueue) EnqueueSendTx(ctx context.Context, _ pgx.Tx, job *port.SendJob) error {
	return q.EnqueueSend(ctx, job)
}

func (q *pipelineQueue) EnqueueWebhook(_ context.Context, _ *port.WebhookJob) error { return nil }
func (q *pipelineQueue) EnqueueWebhookTx(_ context.Context, _ pgx.Tx, _ *port.WebhookJob) error {
	return nil
}

type pipelineCompiler struct{}

func (c *pipelineCompiler) Compile(_ context.Context, mjml string) (string, error) {
	return "<html>" + mjml + "</html>", nil
}

type pipelineRateLimiter struct {
	acquireBurstFn    func(ctx context.Context, adapterID uuid.UUID, requested int) (int, error)
	acquireBurstCalls []pipelineAcquireBurstCall
}

type pipelineAcquireBurstCall struct {
	AdapterID uuid.UUID
	Requested int
}

func (r *pipelineRateLimiter) AcquireBurst(ctx context.Context, adapterID uuid.UUID, requested int) (int, error) {
	r.acquireBurstCalls = append(r.acquireBurstCalls, pipelineAcquireBurstCall{AdapterID: adapterID, Requested: requested})
	if r.acquireBurstFn != nil {
		return r.acquireBurstFn(ctx, adapterID, requested)
	}
	return requested, nil
}

func (r *pipelineRateLimiter) TryAcquire(ctx context.Context, adapterID uuid.UUID) (bool, error) {
	reserved, err := r.AcquireBurst(ctx, adapterID, 1)
	if err != nil {
		return false, err
	}
	return reserved > 0, nil
}

func (r *pipelineRateLimiter) SyncBucket(_ context.Context, _ uuid.UUID, _ int) error { return nil }

type pipelineSender struct {
	sendFn func(ctx context.Context, msg *port.OutgoingEmail) (string, error)
	calls  []*port.OutgoingEmail
}

func (s *pipelineSender) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	cloned := *msg
	cloned.CC = append([]port.EmailAddress(nil), msg.CC...)
	cloned.BCC = append([]port.EmailAddress(nil), msg.BCC...)
	s.calls = append(s.calls, &cloned)
	if s.sendFn != nil {
		return s.sendFn(ctx, msg)
	}
	return "provider-msg-" + uuid.Must(uuid.NewV7()).String(), nil
}

func (s *pipelineSender) Name() string                        { return "pipeline-sender" }
func (s *pipelineSender) HealthCheck(_ context.Context) error { return nil }

func TestSendPipeline_ReworkFlow_BatchesSuppressionsPersistsHotColdAndReusesBurstReservation(t *testing.T) {
	f := newSendFixture()
	emailStore := newPipelineEmailStore()
	queue := &pipelineQueue{}

	suppressionCalls, suppressionInputs := configurePipelineSuppressionBatch(t, f)

	svc := newPipelineSendService(f, emailStore, queue)

	firstResp, err := svc.Send(context.Background(), &service.SendRequest{
		Ref: "latam:acme:welcome",
		To:  []string{"active@example.com", "blocked@example.com"},
		CC:  []string{"cc-blocked@example.com", "cc-allowed@example.com"},
		BCC: []string{"bcc-allowed@example.com"},
		Variables: map[string]any{
			"name": "Ada",
		},
	})
	if err != nil {
		t.Fatalf("unexpected send error: %v", err)
	}

	assertPipelineSuppressionLookup(t, *suppressionCalls, suppressionInputs, []string{
		"active@example.com",
		"blocked@example.com",
		"cc-blocked@example.com",
		"cc-allowed@example.com",
		"bcc-allowed@example.com",
	})

	acceptedTrackingID := assertPipelineFirstSend(t, firstResp, queue)
	assertPipelineColdHotPersistence(t, emailStore, acceptedTrackingID)

	secondResp, err := svc.Send(context.Background(), &service.SendRequest{
		Ref: "latam:acme:welcome",
		To:  []string{"second@example.com"},
		Variables: map[string]any{
			"name": "Grace",
		},
	})
	if err != nil {
		t.Fatalf("unexpected second send error: %v", err)
	}
	assertPipelineSecondSend(t, secondResp, queue)

	rateLimiter := &pipelineRateLimiter{
		acquireBurstFn: func(_ context.Context, adapterID uuid.UUID, requested int) (int, error) {
			if adapterID != f.adapterID {
				t.Fatalf("expected adapter %s, got %s", f.adapterID, adapterID)
			}
			if requested < 2 {
				t.Fatalf("expected burst request >= 2, got %d", requested)
			}
			return 2, nil
		},
	}
	sender := &pipelineSender{}
	worker := riveradapter.NewSendWorker(emailStore, &pipelineCompiler{}, service.NewVariableRenderer(), rateLimiter, sender)

	runPipelineWorkerJobs(t, worker, queue.sendJobs)
	assertPipelineWorkerEffects(t, emailStore, rateLimiter, sender)
}

func configurePipelineSuppressionBatch(t *testing.T, f *sendTestFixture) (*int, *[][]string) {
	t.Helper()

	var calls int
	var inputs [][]string
	f.suppression.checkBatchFn = func(_ context.Context, wsID uuid.UUID, emails []string) (map[string]string, error) {
		if wsID != f.workspaceID {
			t.Fatalf("expected workspace %s, got %s", f.workspaceID, wsID)
		}
		calls++
		inputs = append(inputs, append([]string(nil), emails...))
		return map[string]string{
			"blocked@example.com":    "manual",
			"cc-blocked@example.com": "manual",
		}, nil
	}

	return &calls, &inputs
}

func assertPipelineSuppressionLookup(t *testing.T, calls int, inputs *[][]string, expected []string) {
	t.Helper()

	if calls != 1 {
		t.Fatalf("expected 1 suppression batch lookup, got %d", calls)
	}
	if got := (*inputs)[0]; strings.Join(got, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected suppression lookup recipients: got %v want %v", got, expected)
	}
}

func assertPipelineFirstSend(t *testing.T, resp *service.SendResponse, queue *pipelineQueue) string {
	t.Helper()

	if len(resp.TrackingIDs) != 2 {
		t.Fatalf("expected 2 tracking entries, got %d", len(resp.TrackingIDs))
	}
	if len(queue.sendJobs) != 1 {
		t.Fatalf("expected only active recipient to be enqueued, got %d jobs", len(queue.sendJobs))
	}

	var acceptedTrackingID string
	for _, item := range resp.TrackingIDs {
		switch item.To {
		case "active@example.com":
			if item.Status != "accepted" {
				t.Fatalf("expected active recipient accepted, got %q", item.Status)
			}
			acceptedTrackingID = item.TrackingID
		case "blocked@example.com":
			if item.Status != "suppressed" {
				t.Fatalf("expected blocked recipient suppressed, got %q", item.Status)
			}
		default:
			t.Fatalf("unexpected tracking entry recipient %q", item.To)
		}
	}
	if acceptedTrackingID == "" {
		t.Fatal("expected accepted tracking ID")
	}
	return acceptedTrackingID
}

func assertPipelineColdHotPersistence(t *testing.T, emailStore *pipelineEmailStore, acceptedTrackingID string) {
	t.Helper()

	acceptedHot := emailStore.hotByTrackingID(acceptedTrackingID)
	if acceptedHot == nil {
		t.Fatal("expected accepted hot row to exist")
	}
	if acceptedHot.BodyMJML != "" {
		t.Fatalf("expected hot row body to be empty, got %q", acceptedHot.BodyMJML)
	}
	if acceptedHot.VariablesSnapshot != nil {
		t.Fatalf("expected hot row variables snapshot to be nil, got %#v", acceptedHot.VariablesSnapshot)
	}
	if acceptedHot.InjectorsSnapshot != nil {
		t.Fatalf("expected hot row injector snapshot to be nil, got %#v", acceptedHot.InjectorsSnapshot)
	}

	acceptedPayload := emailStore.payloadByID(acceptedHot.ID)
	if acceptedPayload == nil {
		t.Fatal("expected cold payload row to exist")
	}
	if !strings.Contains(acceptedPayload.BodyMJML, "Hello {{ event.name }}") {
		t.Fatalf("expected cold payload body snapshot, got %q", acceptedPayload.BodyMJML)
	}
}

func assertPipelineSecondSend(t *testing.T, resp *service.SendResponse, queue *pipelineQueue) {
	t.Helper()

	if len(resp.TrackingIDs) != 1 || resp.TrackingIDs[0].Status != "accepted" {
		t.Fatalf("expected second send accepted, got %#v", resp.TrackingIDs)
	}
	if len(queue.sendJobs) != 2 {
		t.Fatalf("expected 2 queued jobs after second send, got %d", len(queue.sendJobs))
	}
}

func runPipelineWorkerJobs(t *testing.T, worker *riveradapter.SendWorker, jobs []*port.SendJob) {
	t.Helper()

	for _, sendJob := range jobs {
		job := &goriver.Job[riveradapter.SendJobArgs]{
			Args: riveradapter.SendJobArgs{
				EmailID:    sendJob.EmailID,
				TrackingID: sendJob.TrackingID,
				AdapterID:  sendJob.AdapterID,
			},
			JobRow: &rivertype.JobRow{
				Attempt:     1,
				MaxAttempts: 5,
			},
		}

		if err := worker.Work(context.Background(), job); err != nil {
			t.Fatalf("unexpected worker error for %s: %v", sendJob.TrackingID, err)
		}
	}
}

func assertPipelineWorkerEffects(t *testing.T, emailStore *pipelineEmailStore, rateLimiter *pipelineRateLimiter, sender *pipelineSender) {
	t.Helper()

	if len(rateLimiter.acquireBurstCalls) != 1 {
		t.Fatalf("expected one burst acquisition for two jobs on same adapter, got %d", len(rateLimiter.acquireBurstCalls))
	}
	if len(emailStore.getPayloadCalls) != 2 {
		t.Fatalf("expected 2 cold payload loads for 2 processed jobs, got %d", len(emailStore.getPayloadCalls))
	}
	if len(sender.calls) != 2 {
		t.Fatalf("expected 2 sends, got %d", len(sender.calls))
	}

	firstSend := sender.calls[0]
	if firstSend.To.Address != "active@example.com" {
		t.Fatalf("expected first send to active recipient, got %q", firstSend.To.Address)
	}
	if len(firstSend.CC) != 1 || firstSend.CC[0].Address != "cc-allowed@example.com" {
		t.Fatalf("expected filtered CC list, got %#v", firstSend.CC)
	}
	if len(firstSend.BCC) != 1 || firstSend.BCC[0].Address != "bcc-allowed@example.com" {
		t.Fatalf("expected BCC preserved, got %#v", firstSend.BCC)
	}
	if !strings.Contains(firstSend.BodyHTML, "Hello Ada") {
		t.Fatalf("expected compiled body from cold payload, got %q", firstSend.BodyHTML)
	}
}

func TestSendPipeline_ReworkFlow_RateLimitedSkipsColdPayload(t *testing.T) {
	f := newSendFixture()
	emailStore := newPipelineEmailStore()
	queue := &pipelineQueue{}
	svc := newPipelineSendService(f, emailStore, queue)

	resp, err := svc.Send(context.Background(), &service.SendRequest{
		Ref: "latam:acme:welcome",
		To:  []string{"rate-limited@example.com"},
		Variables: map[string]any{
			"name": "Linus",
		},
	})
	if err != nil {
		t.Fatalf("unexpected send error: %v", err)
	}
	if len(resp.TrackingIDs) != 1 || resp.TrackingIDs[0].Status != "accepted" {
		t.Fatalf("expected accepted send response, got %#v", resp.TrackingIDs)
	}
	if len(queue.sendJobs) != 1 {
		t.Fatalf("expected 1 queued job, got %d", len(queue.sendJobs))
	}

	rateLimiter := &pipelineRateLimiter{
		acquireBurstFn: func(_ context.Context, _ uuid.UUID, _ int) (int, error) {
			return 0, nil
		},
	}
	sender := &pipelineSender{}
	worker := riveradapter.NewSendWorker(emailStore, &pipelineCompiler{}, service.NewVariableRenderer(), rateLimiter, sender)

	sendJob := queue.sendJobs[0]
	job := &goriver.Job[riveradapter.SendJobArgs]{
		Args: riveradapter.SendJobArgs{
			EmailID:    sendJob.EmailID,
			TrackingID: sendJob.TrackingID,
			AdapterID:  sendJob.AdapterID,
		},
		JobRow: &rivertype.JobRow{
			Attempt:     1,
			MaxAttempts: 5,
		},
	}

	err = worker.Work(context.Background(), job)
	if err == nil {
		t.Fatal("expected rate-limited worker error, got nil")
	}

	var snoozeErr *goriver.JobSnoozeError
	if !errors.As(err, &snoozeErr) {
		t.Fatalf("expected JobSnoozeError, got %T: %v", err, err)
	}
	if len(emailStore.getPayloadCalls) != 0 {
		t.Fatalf("expected no cold payload loads when rate limited, got %d", len(emailStore.getPayloadCalls))
	}
	if len(sender.calls) != 0 {
		t.Fatalf("expected no sends when rate limited, got %d", len(sender.calls))
	}

	hot := emailStore.hotByTrackingID(resp.TrackingIDs[0].TrackingID)
	if hot == nil {
		t.Fatal("expected hot row to exist")
	}
	if hot.Status != domain.StatusQueued {
		t.Fatalf("expected queued status to remain when rate limited, got %s", hot.Status)
	}
}

func newPipelineSendService(f *sendTestFixture, emailStore port.EmailStore, queue port.JobQueue) *service.SendService {
	chainResolver := resolution.NewChainResolver(f.wsStore, f.cache)
	templateResolver := resolution.NewTemplateResolver(f.templateStore, f.cache, chainResolver)
	injectorMerger := resolution.NewInjectorMerger(f.injectorStore, chainResolver, nil, nil, nil)
	adapterResolver := resolution.NewAdapterResolver(f.adapterStore, f.cache)
	renderer := service.NewVariableRenderer()
	identitySvc := service.NewIdentityService(f.identityStore, f.adapterStore, nil, nil)

	return service.NewSendService(
		templateResolver,
		injectorMerger,
		adapterResolver,
		identitySvc,
		emailStore,
		f.suppression,
		queue,
		renderer,
		f.tenantStore,
		f.wsStore,
		nil,
	)
}

func clonePipelineEmail(email *domain.Email) *domain.Email {
	cloned := *email
	cloned.CC = append([]string(nil), email.CC...)
	cloned.BCC = append([]string(nil), email.BCC...)
	cloned.VariablesSnapshot = clonePipelineVars(email.VariablesSnapshot)
	cloned.InjectorsSnapshot = clonePipelineInjectors(email.InjectorsSnapshot)
	return &cloned
}

func clonePipelineVars(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func clonePipelineInjectors(src map[string]map[string]any) map[string]map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]map[string]any, len(src))
	for name, fields := range src {
		fieldCopy := make(map[string]any, len(fields))
		for key, value := range fields {
			fieldCopy[key] = value
		}
		dst[name] = fieldCopy
	}
	return dst
}
