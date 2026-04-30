package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/metrics"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
)

// SendRequest represents an email send API request.
type SendRequest struct {
	Ref        string                    `json:"ref"`
	To         []string                  `json:"to"`
	CC         []string                  `json:"cc,omitempty"`
	BCC        []string                  `json:"bcc,omitempty"`
	Variables  map[string]any            `json:"variables"`
	Injectors  map[string]map[string]any `json:"injectors,omitempty"`
	ExternalID *string                   `json:"external_id,omitempty"`
	Locale     *string                   `json:"locale,omitempty"`
	// AuthWorkspaceID is the workspace resolved from API key auth context.
	// When set, it must match the workspace resolved from Ref.
	AuthWorkspaceID uuid.UUID `json:"-"`
	// Headers carries HTTP request headers for code injectors. Optional.
	Headers map[string]string `json:"-"`
	// Source tracks where this logical send originated from.
	Source SendSource `json:"-"`
}

// SendBatchRequest represents a batch email send API request for a single template ref.
type SendBatchRequest struct {
	Ref string `json:"ref"`

	Items []SendBatchItemRequest `json:"items"`

	AuthWorkspaceID uuid.UUID         `json:"-"`
	Headers         map[string]string `json:"-"`
	Source          SendSource        `json:"-"`
}

// SendBatchItemRequest represents one logical message in a batch send.
type SendBatchItemRequest struct {
	To         string                    `json:"to"`
	CC         []string                  `json:"cc,omitempty"`
	BCC        []string                  `json:"bcc,omitempty"`
	Variables  map[string]any            `json:"variables,omitempty"`
	Injectors  map[string]map[string]any `json:"injectors,omitempty"`
	ExternalID *string                   `json:"external_id,omitempty"`
	Locale     *string                   `json:"locale,omitempty"`
}

// SendSource captures provenance for persisted emails.
type SendSource struct {
	Type          domain.EmailSourceType `json:"type"`
	ActorMemberID *uuid.UUID             `json:"actor_member_id,omitempty"`
	ActorEmail    *string                `json:"actor_email,omitempty"`
}

// SendResponse represents the result of a send operation.
type SendResponse struct {
	Status           string          `json:"status"`
	TrackingIDs      []TrackingEntry `json:"tracking_ids"`
	ExternalID       *string         `json:"external_id,omitempty"`
	TemplateResolved string          `json:"template_resolved"`
	TemplateVersion  int             `json:"template_version"`
}

// SendBatchResponse represents the result of a batch send operation.
type SendBatchResponse struct {
	Status           string                `json:"status"`
	TemplateResolved string                `json:"template_resolved"`
	Items            []SendBatchItemResult `json:"items"`
	AcceptedCount    int                   `json:"accepted_count"`
	SuppressedCount  int                   `json:"suppressed_count"`
	FailedCount      int                   `json:"failed_count"`
}

// SendBatchItemResult contains the outcome of one batch item.
type SendBatchItemResult struct {
	Index      int     `json:"index"`
	To         string  `json:"to"`
	TrackingID string  `json:"tracking_id,omitempty"`
	Status     string  `json:"status"`
	ExternalID *string `json:"external_id,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// TrackingEntry maps a recipient to their tracking ID and per-recipient status.
type TrackingEntry struct {
	To         string `json:"to"`
	TrackingID string `json:"tracking_id"`
	Status     string `json:"status"`          // "accepted", "suppressed", or "failed"
	Error      string `json:"error,omitempty"` // populated when Status is "failed"
}

// SendService orchestrates the full email send pipeline.
type SendService struct {
	templateResolver            *resolution.TemplateResolver
	injectorMerger              *resolution.InjectorMerger
	adapterResolver             *resolution.AdapterResolver
	accessService               *AdapterAccessService
	identitySvc                 *IdentityService
	emailStore                  port.EmailStore
	suppression                 port.SuppressionStore
	templateTypeSubscriptionStore templateTypeOptOutStore
	queue                       port.JobQueue
	renderer                    port.VariableRenderer
	tenantStore                 port.TenantStore
	wsStore                     port.WorkspaceStore
	pool                        *pgxpool.Pool
}

// SetAdapterAccessService wires runtime adapter access validation without widening constructor churn.
func (s *SendService) SetAdapterAccessService(accessService *AdapterAccessService) {
	s.accessService = accessService
}

// SetTemplateTypeSubscriptionStore wires per-type opt-out suppression without widening constructor churn.
func (s *SendService) SetTemplateTypeSubscriptionStore(ts templateTypeOptOutStore) {
	s.templateTypeSubscriptionStore = ts
}

// NewSendService creates a new SendService with the given dependencies.
func NewSendService(
	templateResolver *resolution.TemplateResolver,
	injectorMerger *resolution.InjectorMerger,
	adapterResolver *resolution.AdapterResolver,
	identitySvc *IdentityService,
	emailStore port.EmailStore,
	suppression port.SuppressionStore,
	queue port.JobQueue,
	renderer port.VariableRenderer,
	tenantStore port.TenantStore,
	wsStore port.WorkspaceStore,
	pool *pgxpool.Pool,
) *SendService {
	return &SendService{
		templateResolver: templateResolver,
		injectorMerger:   injectorMerger,
		adapterResolver:  adapterResolver,
		identitySvc:      identitySvc,
		emailStore:       emailStore,
		suppression:      suppression,
		queue:            queue,
		renderer:         renderer,
		tenantStore:      tenantStore,
		wsStore:          wsStore,
		pool:             pool,
	}
}

// Send orchestrates the full email send pipeline:
// 1. Parse ref → 2. Resolve tenant/workspace → 3. Resolve template →
// 4. Merge injectors → 5. Resolve adapter → 6. Resolve from_email →
// 7. Render fields → 8. Create emails → 9. Enqueue jobs
func (s *SendService) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) { //nolint:gocognit,gocyclo,funlen // multi-step email send orchestration pipeline
	// 1. Parse addressing ref (tenant:workspace:templateType)
	ref, err := domain.ParseRef(req.Ref)
	if err != nil {
		return nil, err
	}

	// 2. Resolve tenant and workspace
	tenant, err := s.tenantStore.GetByCode(ctx, ref.TenantCode)
	if err != nil {
		return nil, err
	}

	environment := domain.EnvironmentProd
	if req.AuthWorkspaceID != uuid.Nil {
		authWorkspace, err := s.wsStore.GetByID(ctx, req.AuthWorkspaceID)
		if err != nil {
			return nil, err
		}
		environment = authWorkspace.Environment
		if !environment.Valid() {
			environment = domain.EnvironmentProd
		}
	}

	ws, err := s.wsStore.GetByTenantAndCode(ctx, tenant.ID, ref.WorkspaceCode, environment)
	if err != nil {
		return nil, err
	}

	if req.AuthWorkspaceID != uuid.Nil && ws.ID != req.AuthWorkspaceID {
		slog.Warn("send rejected", "reason", "scope_mismatch", "auth_workspace_id", req.AuthWorkspaceID, "ref_workspace_id", ws.ID)
		return nil, domain.ErrWorkspaceScopeMismatch
	}

	if ws.IsSystem {
		slog.Warn("send rejected", "reason", "system_workspace_blocked", "workspace_id", ws.ID)
		return nil, domain.ErrSystemWorkspaceBlocked
	}

	// 3. Resolve template (includes kill switch check, published version, locale fallback).
	// If caller didn't specify a locale, fall back to workspace default.
	locale := req.Locale
	if (locale == nil || *locale == "") && ws.DefaultLocale != nil && *ws.DefaultLocale != "" {
		locale = ws.DefaultLocale
	}
	resolved, err := s.templateResolver.Resolve(ctx, ws.ID, ref.TemplateType, locale)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateDisabled) {
			slog.Warn("send rejected", "reason", "template_disabled", "template_type", ref.TemplateType, "workspace_id", ws.ID)
		}
		return nil, err
	}

	// 4. Merge injectors (workspace defaults + request overrides + optional code injectors).
	injCtx := port.NewInjectorContext(
		req.Headers,
		req.Ref,
		req.Variables,
		tenant.ID, ws.ID,
		ws.Environment,
		ref.TemplateType,
	)
	injCtx.SetRequestInjectors(req.Injectors)
	injectors, err := s.injectorMerger.ResolveWithContext(ctx, ws.ID, injCtx)
	if err != nil {
		return nil, err
	}

	// 5. Resolve adapter (from template type assignment)
	adapter, err := s.adapterResolver.ResolveForTemplateType(ctx, resolved.TemplateType)
	if err != nil {
		return nil, err
	}

	if s.accessService != nil {
		if err := s.accessService.ValidateTemplateTypeSelection(ctx, ws, resolved.TemplateType.AdapterID, resolved.TemplateType.SenderIdentityID); err != nil {
			return nil, err
		}
	}

	// 6. Resolve from_email — use template type's sender identity if set, else adapter default.
	fromEmail, err := s.resolveFromEmail(ctx, adapter.Adapter, resolved.TemplateType.SenderIdentityID)
	if err != nil {
		return nil, err
	}

	// 8. Render subject, preview text, from name with variables
	subject := getLocalizedField(resolved, "subject")
	fromName := getLocalizedField(resolved, "from_name")

	renderedSubject, err := s.renderer.Render(subject, injectors, req.Variables)
	if err != nil {
		return nil, fmt.Errorf("render subject: %w", err)
	}
	renderedFromName, err := s.renderer.Render(fromName, injectors, req.Variables)
	if err != nil {
		return nil, fmt.Errorf("render from_name: %w", err)
	}

	// 9. Get the MJML body (locale-aware)
	bodyMJML := getLocalizedBody(resolved)

	// 10. Create email records and enqueue jobs
	now := time.Now().UTC()
	source := effectiveSendSource(req.Source)
	response := &SendResponse{
		Status:           "accepted",
		TemplateResolved: req.Ref,
		TemplateVersion:  resolved.Version.VersionNumber,
		ExternalID:       req.ExternalID,
	}

	var failCount int
	var lastErr error

	effectiveTo, effectiveCC, effectiveBCC, err := applyTestRecipientPolicy(ws, resolved.TemplateType, req.To, req.CC, req.BCC)
	if err != nil {
		return nil, err
	}

	suppressionResult, err := NewSuppressionBatchEvaluator(s.suppression).
		WithTemplateTypeStore(s.templateTypeSubscriptionStore).
		EvaluateForType(ctx, ws.ID, resolved.TemplateType.ID, effectiveTo, effectiveCC, effectiveBCC)
	if err != nil {
		return nil, err
	}

	for _, recipient := range suppressionResult.To {

		trackingID := generateTrackingID()

		email := &domain.Email{
			ID:                  uuid.Must(uuid.NewV7()),
			TrackingID:          trackingID,
			ExternalID:          req.ExternalID,
			WorkspaceID:         ws.ID,
			TenantID:            tenant.ID,
			TemplateID:          resolved.Template.ID,
			TemplateVersionID:   resolved.Version.ID,
			TemplateTypeSlug:    ref.TemplateType,
			TemplateRef:         req.Ref,
			RecipientEmail:      recipient.Address,
			CC:                  suppressionResult.CC,
			BCC:                 suppressionResult.BCC,
			FromEmail:           fromEmail,
			FromName:            renderedFromName,
			ReplyTo:             resolved.Version.ReplyTo,
			SubjectRendered:     renderedSubject,
			Locale:              req.Locale,
			AdapterID:           adapter.Adapter.ID,
			SenderIdentityID:    resolved.TemplateType.SenderIdentityID,
			VariablesSnapshot:   req.Variables,
			InjectorsSnapshot:   injectors,
			SourceType:          source.Type,
			SourceActorMemberID: source.ActorMemberID,
			SourceActorEmail:    source.ActorEmail,
			BodyMJML:            bodyMJML,
			OpenTrackingEnabled: ws.OpenTrackingEnabled,
			MaxRetries:          3,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if recipient.Suppressed {
			email.Status = domain.StatusSuppressed
			if err := s.createSuppressed(ctx, email, now, recipient.Reason); err != nil {
				failCount++
				lastErr = err
				slog.Error("failed to create suppressed email", "recipient", recipient.Address, "error", err)
				response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
					To:         recipient.Address,
					TrackingID: trackingID,
					Status:     "failed",
					Error:      err.Error(),
				})
				continue
			}
			response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
				To:         recipient.Address,
				TrackingID: trackingID,
				Status:     "suppressed",
			})
		} else {
			email.Status = domain.StatusQueued

			if createErr := s.createAndEnqueue(ctx, email, trackingID, adapter.Adapter.ID); createErr != nil {
				failCount++
				lastErr = createErr
				slog.Error("failed to create and enqueue email", "recipient", recipient.Address, "error", createErr)
				response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
					To:         recipient.Address,
					TrackingID: trackingID,
					Status:     "failed",
					Error:      createErr.Error(),
				})
				continue
			}

			metrics.EmailsEnqueued.Inc()
			response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
				To:         recipient.Address,
				TrackingID: trackingID,
				Status:     "accepted",
			})
		}
	}

	// If ALL recipients failed, return the last error.
	if failCount == len(effectiveTo) {
		return nil, fmt.Errorf("all recipients failed: %w", lastErr)
	}

	// Adjust overall status if some recipients failed.
	if failCount > 0 {
		response.Status = "partial"
	}

	return response, nil
}

func effectiveSendSource(source SendSource) SendSource {
	if source.Type == "" {
		source.Type = domain.EmailSourceTypeDataPlaneAPIKey
	}
	return source
}

func applyTestRecipientPolicy(
	ws *domain.Workspace,
	templateType *domain.TemplateType,
	to []string,
	cc []string,
	bcc []string,
) ([]string, []string, []string, error) {
	if ws == nil || ws.Environment != domain.EnvironmentTest {
		return to, cc, bcc, nil
	}

	mode := ws.TestRecipientMode
	if !mode.Valid() {
		mode = domain.TestRecipientModeReplace
	}
	recipients := domain.NormalizeRecipientAddresses(ws.TestRecipientAddresses)

	if templateType != nil && templateType.TestRecipientMode != nil && templateType.TestRecipientMode.Valid() {
		mode = *templateType.TestRecipientMode
		recipients = domain.NormalizeRecipientAddresses(templateType.TestRecipientAddresses)
	}

	if len(recipients) == 0 {
		return nil, nil, nil, domain.ErrTestRecipientPolicyUnconfigured
	}

	switch mode {
	case domain.TestRecipientModeAppend:
		return append(domain.NormalizeRecipientAddresses(to), recipients...), cc, bcc, nil
	case domain.TestRecipientModeReplace:
		fallthrough
	default:
		return recipients, nil, nil, nil
	}
}

// resolveFromEmail gets the from_email. If senderIdentityID is set, uses that specific identity;
// otherwise falls back to the adapter's default identity.
func (s *SendService) resolveFromEmail(ctx context.Context, adapter *domain.Adapter, senderIdentityID *uuid.UUID) (string, error) {
	return resolveFromEmail(s.identitySvc.identityStore, ctx, adapter, senderIdentityID)
}

// getLocalizedField returns the localized value for a field, falling back to version default.
func getLocalizedField(resolved *resolution.ResolvedTemplate, field string) string {
	if resolved.Locale != nil {
		switch field {
		case "subject":
			if resolved.Locale.Subject != nil {
				return *resolved.Locale.Subject
			}
		case "preview_text":
			if resolved.Locale.PreviewText != nil {
				return *resolved.Locale.PreviewText
			}
		case "from_name":
			if resolved.Locale.FromName != nil {
				return *resolved.Locale.FromName
			}
		}
	}

	// Fallback to version defaults
	switch field {
	case "subject":
		return resolved.Version.Subject
	case "preview_text":
		return resolved.Version.PreviewText
	case "from_name":
		return resolved.Version.FromName
	default:
		return ""
	}
}

// createAndEnqueue atomically creates an email record, records a "queued" event,
// and enqueues a send job. When a pool is available, all three operations are
// wrapped in a DB transaction. Falls back to non-transactional path when pool
// is nil (e.g., in unit tests).
func (s *SendService) createAndEnqueue(ctx context.Context, email *domain.Email, trackingID string, adapterID uuid.UUID) error {
	return s.persistenceWriter().CreateQueued(ctx, email, trackingID, adapterID)
}

// createSuppressed atomically creates the email record and the suppression event.
// When a pool is available, both writes are wrapped in a DB transaction so they
// cannot be partially applied. Falls back to non-transactional path when pool is
// nil (e.g., in unit tests).
func (s *SendService) createSuppressed(ctx context.Context, email *domain.Email, now time.Time, reason string) error {
	return s.persistenceWriter().CreateSuppressed(ctx, email, now, reason)
}

// getLocalizedBody returns the MJML body, preferring locale override if present.
func getLocalizedBody(resolved *resolution.ResolvedTemplate) string {
	if resolved.Locale != nil && resolved.Locale.BodyMJML != nil {
		return *resolved.Locale.BodyMJML
	}
	return resolved.Version.BodyMJML
}

func (s *SendService) persistenceWriter() *SendPersistenceWriter {
	return NewSendPersistenceWriter(s.emailStore, s.queue, s.pool)
}

// generateTrackingID creates a "trk_" prefixed unique tracking ID.
func generateTrackingID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to UUID if crypto/rand fails (should never happen)
		return fmt.Sprintf("trk_%s", uuid.Must(uuid.NewV7()).String())
	}
	return "trk_" + hex.EncodeToString(b)
}
