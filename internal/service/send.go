package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/resolution"
)

// SendRequest represents an email send API request.
type SendRequest struct {
	Ref        string         `json:"ref"`
	To         []string       `json:"to"`
	CC         []string       `json:"cc,omitempty"`
	BCC        []string       `json:"bcc,omitempty"`
	Variables  map[string]any `json:"variables"`
	ExternalID *string        `json:"external_id,omitempty"`
	Locale     *string        `json:"locale,omitempty"`
}

// SendResponse represents the result of a send operation.
type SendResponse struct {
	Status           string          `json:"status"`
	TrackingIDs      []TrackingEntry `json:"tracking_ids"`
	ExternalID       *string         `json:"external_id,omitempty"`
	TemplateResolved string          `json:"template_resolved"`
	TemplateVersion  int             `json:"template_version"`
}

// TrackingEntry maps a recipient to their tracking ID and per-recipient status.
type TrackingEntry struct {
	To         string `json:"to"`
	TrackingID string `json:"tracking_id"`
	Status     string `json:"status"`           // "accepted", "suppressed", or "failed"
	Error      string `json:"error,omitempty"`   // populated when Status is "failed"
}

// SendService orchestrates the full email send pipeline.
type SendService struct {
	templateResolver *resolution.TemplateResolver
	injectorMerger   *resolution.InjectorMerger
	adapterResolver  *resolution.AdapterResolver
	identitySvc      *IdentityService
	emailStore       port.EmailStore
	suppression      port.SuppressionStore
	queue            port.JobQueue
	renderer         port.VariableRenderer
	tenantStore      port.TenantStore
	wsStore          port.WorkspaceStore
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
	}
}

// Send orchestrates the full email send pipeline:
// 1. Parse ref → 2. Resolve tenant/workspace → 3. Resolve template →
// 4. Merge injectors → 5. Resolve adapter → 6. Resolve from_email →
// 7. Render fields → 8. Create emails → 9. Enqueue jobs
func (s *SendService) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
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

	ws, err := s.wsStore.GetByTenantAndCode(ctx, tenant.ID, ref.WorkspaceCode)
	if err != nil {
		return nil, err
	}

	// 3. Resolve template (includes kill switch check, published version, locale fallback)
	resolved, err := s.templateResolver.Resolve(ctx, ws.ID, ref.TemplateType, req.Locale)
	if err != nil {
		return nil, err
	}

	// 4. Merge injectors
	injectors, err := s.injectorMerger.Resolve(ctx, ws.ID)
	if err != nil {
		return nil, err
	}

	// 5. Resolve adapter (from template type assignment)
	adapter, err := s.adapterResolver.ResolveForTemplateType(ctx, resolved.TemplateType)
	if err != nil {
		return nil, err
	}

	// 6. Resolve from_email from adapter's default identity.
	fromEmail, err := s.resolveFromEmail(ctx, adapter.Adapter)
	if err != nil {
		return nil, err
	}

	// 7. Render subject, preview text, from name with variables
	subject := getLocalizedField(resolved, "subject")
	fromName := getLocalizedField(resolved, "from_name")

	renderedSubject, _ := s.renderer.Render(subject, injectors, req.Variables)
	renderedFromName, _ := s.renderer.Render(fromName, injectors, req.Variables)

	// 8. Get the MJML body (locale-aware)
	bodyMJML := getLocalizedBody(resolved)

	// 9. Create email records and enqueue jobs
	now := time.Now().UTC()
	response := &SendResponse{
		Status:           "accepted",
		TemplateResolved: req.Ref,
		TemplateVersion:  resolved.Version.VersionNumber,
		ExternalID:       req.ExternalID,
	}

	var failCount int
	var lastErr error

	for _, recipient := range req.To {
		// Check suppression
		suppressed, reason, err := s.suppression.IsSuppressed(ctx, ws.ID, recipient)
		if err != nil {
			return nil, fmt.Errorf("check suppression for %s: %w", recipient, err)
		}

		trackingID := generateTrackingID()

		email := &domain.Email{
			ID:                uuid.Must(uuid.NewV7()),
			TrackingID:        trackingID,
			ExternalID:        req.ExternalID,
			WorkspaceID:       ws.ID,
			TenantID:          tenant.ID,
			TemplateID:        resolved.Template.ID,
			TemplateVersionID: resolved.Version.ID,
			TemplateTypeSlug:  ref.TemplateType,
			TemplateRef:       req.Ref,
			RecipientEmail:    recipient,
			CC:                req.CC,
			BCC:               req.BCC,
			FromEmail:         fromEmail,
			FromName:          renderedFromName,
			ReplyTo:           resolved.Version.ReplyTo,
			SubjectRendered:   renderedSubject,
			Locale:            req.Locale,
			AdapterID:         adapter.Adapter.ID,
			VariablesSnapshot: req.Variables,
			InjectorsSnapshot: injectors,
			BodyMJML:          bodyMJML,
			MaxRetries:        3,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		if suppressed {
			email.Status = domain.StatusSuppressed
			if err := s.emailStore.Create(ctx, email); err != nil {
				failCount++
				lastErr = err
				slog.Error("failed to create suppressed email", "recipient", recipient, "error", err)
				response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
					To:         recipient,
					TrackingID: trackingID,
					Status:     "failed",
					Error:      err.Error(),
				})
				continue
			}
			if err := s.emailStore.AddEvent(ctx, &domain.EmailEvent{
				ID:         uuid.Must(uuid.NewV7()),
				EmailID:    email.ID,
				EventType:  domain.StatusSuppressed,
				OccurredAt: now,
				Metadata:   map[string]any{"reason": reason},
				CreatedAt:  now,
			}); err != nil {
				slog.Error("failed to add suppression event", "email_id", email.ID, "error", err)
			}
			response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
				To:         recipient,
				TrackingID: trackingID,
				Status:     "suppressed",
			})
		} else {
			email.Status = domain.StatusQueued
			if err := s.emailStore.Create(ctx, email); err != nil {
				failCount++
				lastErr = err
				slog.Error("failed to create email", "recipient", recipient, "error", err)
				response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
					To:         recipient,
					TrackingID: trackingID,
					Status:     "failed",
					Error:      err.Error(),
				})
				continue
			}
			if err := s.queue.EnqueueSend(ctx, &port.SendJob{
				EmailID:    email.ID,
				TrackingID: trackingID,
				AdapterID:  adapter.Adapter.ID,
			}); err != nil {
				failCount++
				lastErr = err
				slog.Error("failed to enqueue send job", "email_id", email.ID, "recipient", recipient, "error", err)
				response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
					To:         recipient,
					TrackingID: trackingID,
					Status:     "failed",
					Error:      err.Error(),
				})
				continue
			}
			response.TrackingIDs = append(response.TrackingIDs, TrackingEntry{
				To:         recipient,
				TrackingID: trackingID,
				Status:     "accepted",
			})
		}
	}

	// If ALL recipients failed, return the last error.
	if failCount == len(req.To) {
		return nil, fmt.Errorf("all recipients failed: %w", lastErr)
	}

	// Adjust overall status if some recipients failed.
	if failCount > 0 {
		response.Status = "partial"
	}

	return response, nil
}

// resolveFromEmail gets the from_email from the adapter's default identity.
func (s *SendService) resolveFromEmail(ctx context.Context, adapter *domain.Adapter) (string, error) {
	identity, err := s.identitySvc.GetDefault(ctx, adapter.ID)
	if err != nil {
		return "", fmt.Errorf("%w: adapter %s", domain.ErrNoDefaultIdentity, adapter.ID)
	}
	if identity.IdentityType != domain.IdentityTypeEmail {
		return "", fmt.Errorf("%w: adapter %s default is a domain, not an email", domain.ErrNoDefaultIdentity, adapter.ID)
	}
	return identity.Identity, nil
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

// getLocalizedBody returns the MJML body, preferring locale override if present.
func getLocalizedBody(resolved *resolution.ResolvedTemplate) string {
	if resolved.Locale != nil && resolved.Locale.BodyMJML != nil {
		return *resolved.Locale.BodyMJML
	}
	return resolved.Version.BodyMJML
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
