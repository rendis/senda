package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
)

// ResolvedSendContext holds the stable resolution state shared across a batch.
type ResolvedSendContext struct {
	Ref                    string
	Tenant                 *domain.Tenant
	Workspace              *domain.Workspace
	TemplateType           *domain.TemplateType
	Adapter                *resolution.ResolvedAdapter
	FromEmail              string
	Headers                map[string]string
	Source                 SendSource
	suppressionStatuses    map[string]port.SuppressionStatus
	workspaceDefaultLocale *string
	templateCache          map[string]*resolution.ResolvedTemplate
}

// SendPlan captures the per-item variable portion of a batch send.
type SendPlan struct {
	Index      int
	To         string
	CC         []string
	BCC        []string
	Variables  map[string]any
	Injectors  map[string]map[string]any
	ExternalID *string
	Locale     *string
}

func (s *SendService) resolveSendBatchContext(ctx context.Context, req *SendBatchRequest) (*ResolvedSendContext, error) {
	ref, err := domain.ParseRef(req.Ref)
	if err != nil {
		return nil, err
	}

	tenant, ws, resolved, adapter, fromEmail, err := s.resolveStableSendContext(ctx, req.AuthWorkspaceID, req.Headers, req.Source, ref)
	if err != nil {
		return nil, err
	}

	templateCache := map[string]*resolution.ResolvedTemplate{
		"": resolved,
	}
	if key := effectiveLocaleKey(nil, ws.DefaultLocale); key != "" {
		templateCache[key] = resolved
	}

	return &ResolvedSendContext{
		Ref:                    req.Ref,
		Tenant:                 tenant,
		Workspace:              ws,
		TemplateType:           resolved.TemplateType,
		Adapter:                adapter,
		FromEmail:              fromEmail,
		Headers:                req.Headers,
		Source:                 effectiveSendSource(req.Source),
		workspaceDefaultLocale: ws.DefaultLocale,
		templateCache:          templateCache,
	}, nil
}

func (s *SendService) resolveStableSendContext(
	ctx context.Context,
	authWorkspaceID uuid.UUID,
	headers map[string]string,
	source SendSource,
	ref *domain.TemplateRef,
) (*domain.Tenant, *domain.Workspace, *resolution.ResolvedTemplate, *resolution.ResolvedAdapter, string, error) {
	tenant, err := s.tenantStore.GetByCode(ctx, ref.TenantCode)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	environment := domain.EnvironmentProd
	if authWorkspaceID != uuid.Nil {
		authWorkspace, err := s.wsStore.GetByID(ctx, authWorkspaceID)
		if err != nil {
			return nil, nil, nil, nil, "", err
		}
		environment = authWorkspace.Environment
		if !environment.Valid() {
			environment = domain.EnvironmentProd
		}
	}

	ws, err := s.wsStore.GetByTenantAndCode(ctx, tenant.ID, ref.WorkspaceCode, environment)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	if authWorkspaceID != uuid.Nil && ws.ID != authWorkspaceID {
		return nil, nil, nil, nil, "", domain.ErrWorkspaceScopeMismatch
	}
	if ws.IsSystem {
		return nil, nil, nil, nil, "", domain.ErrSystemWorkspaceBlocked
	}

	baseLocale := ws.DefaultLocale
	if baseLocale != nil && strings.TrimSpace(*baseLocale) == "" {
		baseLocale = nil
	}

	resolved, err := s.templateResolver.Resolve(ctx, ws.ID, ref.TemplateType, baseLocale)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	adapter, err := s.adapterResolver.ResolveForTemplateType(ctx, resolved.TemplateType)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	if s.accessService != nil {
		if err := s.accessService.ValidateTemplateTypeSelection(ctx, ws, resolved.TemplateType.AdapterID, resolved.TemplateType.SenderIdentityID); err != nil {
			return nil, nil, nil, nil, "", err
		}
	}

	fromEmail, err := s.resolveFromEmail(ctx, adapter.Adapter, resolved.TemplateType.SenderIdentityID)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	return tenant, ws, resolved, adapter, fromEmail, nil
}

func (c *ResolvedSendContext) templateForLocale(ctx context.Context, s *SendService, locale *string) (*resolution.ResolvedTemplate, error) {
	key := effectiveLocaleKey(locale, c.workspaceDefaultLocale)
	if resolved, ok := c.templateCache[key]; ok {
		return resolved, nil
	}

	effectiveLocale := locale
	if effectiveLocale == nil || strings.TrimSpace(*effectiveLocale) == "" {
		effectiveLocale = c.workspaceDefaultLocale
	}

	resolved, err := s.templateResolver.Resolve(ctx, c.Workspace.ID, c.TemplateType.Slug, effectiveLocale)
	if err != nil {
		return nil, err
	}
	c.templateCache[key] = resolved
	return resolved, nil
}

func effectiveLocaleKey(locale *string, fallback *string) string {
	if locale != nil && strings.TrimSpace(*locale) != "" {
		return *locale
	}
	if fallback != nil && strings.TrimSpace(*fallback) != "" {
		return *fallback
	}
	return ""
}

func (s *SendService) SendBatch(ctx context.Context, req *SendBatchRequest) (*SendBatchResponse, error) {
	shared, err := s.resolveSendBatchContext(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.primeSendBatchSuppressionStatuses(ctx, shared, req.Items); err != nil {
		return nil, err
	}

	resp := &SendBatchResponse{
		Status:           "accepted",
		TemplateResolved: req.Ref,
		Items:            make([]SendBatchItemResult, 0, len(req.Items)),
	}
	fullyFailedItems := 0

	for i, item := range req.Items {
		plan := SendPlan{
			Index:      i,
			To:         item.To,
			CC:         item.CC,
			BCC:        item.BCC,
			Variables:  item.Variables,
			Injectors:  item.Injectors,
			ExternalID: item.ExternalID,
			Locale:     item.Locale,
		}

		result, err := s.executeSendPlan(ctx, shared, plan)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			resp.FailedCount++
			fullyFailedItems++
			resp.Items = append(resp.Items, result)
			continue
		}

		switch result.Status {
		case "accepted":
			resp.AcceptedCount++
		case "suppressed":
			resp.SuppressedCount++
		case "failed":
			resp.FailedCount++
			fullyFailedItems++
		default:
			resp.FailedCount++
		}

		resp.Items = append(resp.Items, result)
	}

	switch {
	case fullyFailedItems == len(req.Items) && len(req.Items) > 0:
		resp.Status = "failed"
	case resp.FailedCount > 0:
		resp.Status = "partial"
	default:
		resp.Status = "accepted"
	}

	return resp, nil
}

func (s *SendService) primeSendBatchSuppressionStatuses(ctx context.Context, shared *ResolvedSendContext, items []SendBatchItemRequest) error {
	relevantAddresses := make([]string, 0, len(items)*3)
	for _, item := range items {
		effectiveTo, effectiveCC, effectiveBCC, err := applyTestRecipientPolicy(shared.Workspace, shared.TemplateType, []string{item.To}, item.CC, item.BCC)
		if err != nil {
			return err
		}
		relevantAddresses = append(relevantAddresses, effectiveTo...)
		relevantAddresses = append(relevantAddresses, effectiveCC...)
		relevantAddresses = append(relevantAddresses, effectiveBCC...)
	}

	statuses, err := s.getSuppressionStatuses(ctx, shared.Workspace.ID, relevantAddresses)
	if err != nil {
		return err
	}
	shared.suppressionStatuses = statuses
	return nil
}

func (s *SendService) executeSendPlan(ctx context.Context, shared *ResolvedSendContext, plan SendPlan) (SendBatchItemResult, error) {
	result := SendBatchItemResult{
		Index:      plan.Index,
		To:         plan.To,
		ExternalID: plan.ExternalID,
	}

	resolved, err := shared.templateForLocale(ctx, s, plan.Locale)
	if err != nil {
		return result, err
	}

	injCtx := port.NewInjectorContext(
		shared.Headers,
		shared.Ref,
		plan.Variables,
		shared.Tenant.ID,
		shared.Workspace.ID,
		shared.Workspace.Environment,
		shared.TemplateType.Slug,
	)
	injCtx.SetRequestInjectors(plan.Injectors)
	injectors, err := s.injectorMerger.ResolveWithContext(ctx, shared.Workspace.ID, injCtx)
	if err != nil {
		return result, err
	}

	subject := getLocalizedField(resolved, "subject")
	fromName := getLocalizedField(resolved, "from_name")

	renderedSubject, err := s.renderer.Render(subject, injectors, plan.Variables)
	if err != nil {
		return result, fmt.Errorf("render subject: %w", err)
	}
	renderedFromName, err := s.renderer.Render(fromName, injectors, plan.Variables)
	if err != nil {
		return result, fmt.Errorf("render from_name: %w", err)
	}

	bodyMJML := getLocalizedBody(resolved)

	now := time.Now().UTC()
	source := shared.Source
	effectiveTo, effectiveCC, effectiveBCC, err := applyTestRecipientPolicy(shared.Workspace, resolved.TemplateType, []string{plan.To}, plan.CC, plan.BCC)
	if err != nil {
		return result, err
	}

	filteredCC := filterSuppressedWithStatuses(effectiveCC, shared.suppressionStatuses)
	filteredBCC := filterSuppressedWithStatuses(effectiveBCC, shared.suppressionStatuses)

	var lastErr error
	var failCount int
	var firstTracking TrackingEntry
	var hasFirstTracking bool

	for _, recipient := range effectiveTo {
		status := shared.suppressionStatuses[recipient]
		suppressed := status.Suppressed
		reason := status.Reason

		trackingID := generateTrackingID()
		email := &domain.Email{
			ID:                  uuid.Must(uuid.NewV7()),
			TrackingID:          trackingID,
			ExternalID:          plan.ExternalID,
			WorkspaceID:         shared.Workspace.ID,
			TenantID:            shared.Tenant.ID,
			TemplateID:          resolved.Template.ID,
			TemplateVersionID:   resolved.Version.ID,
			TemplateTypeSlug:    shared.TemplateType.Slug,
			TemplateRef:         shared.Ref,
			RecipientEmail:      recipient,
			CC:                  filteredCC,
			BCC:                 filteredBCC,
			FromEmail:           shared.FromEmail,
			FromName:            renderedFromName,
			ReplyTo:             resolved.Version.ReplyTo,
			SubjectRendered:     renderedSubject,
			Locale:              plan.Locale,
			AdapterID:           shared.Adapter.Adapter.ID,
			SenderIdentityID:    resolved.TemplateType.SenderIdentityID,
			VariablesSnapshot:   plan.Variables,
			InjectorsSnapshot:   injectors,
			SourceType:          source.Type,
			SourceActorMemberID: source.ActorMemberID,
			SourceActorEmail:    source.ActorEmail,
			BodyMJML:            bodyMJML,
			OpenTrackingEnabled: shared.Workspace.OpenTrackingEnabled,
			MaxRetries:          3,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		if suppressed {
			email.Status = domain.StatusSuppressed
			if err := s.createSuppressed(ctx, email, now, reason); err != nil {
				failCount++
				lastErr = err
				if !hasFirstTracking {
					firstTracking = TrackingEntry{To: recipient, TrackingID: trackingID, Status: "failed", Error: err.Error()}
					hasFirstTracking = true
				}
				continue
			}
			if !hasFirstTracking {
				firstTracking = TrackingEntry{To: recipient, TrackingID: trackingID, Status: "suppressed"}
				hasFirstTracking = true
			}
			continue
		}

		email.Status = domain.StatusQueued
		if err := s.createAndEnqueue(ctx, email, trackingID, shared.Adapter.Adapter.ID); err != nil {
			failCount++
			lastErr = err
			if !hasFirstTracking {
				firstTracking = TrackingEntry{To: recipient, TrackingID: trackingID, Status: "failed", Error: err.Error()}
				hasFirstTracking = true
			}
			continue
		}

		if !hasFirstTracking {
			firstTracking = TrackingEntry{To: recipient, TrackingID: trackingID, Status: "accepted"}
			hasFirstTracking = true
		}
	}

	if failCount == len(effectiveTo) {
		return result, fmt.Errorf("all recipients failed: %w", lastErr)
	}

	if failCount > 0 {
		result.Status = "partial"
		if lastErr != nil {
			result.Error = lastErr.Error()
		}
	} else if hasFirstTracking {
		result.Status = firstTracking.Status
		result.TrackingID = firstTracking.TrackingID
		result.Error = firstTracking.Error
		return result, nil
	}

	if hasFirstTracking {
		result.TrackingID = firstTracking.TrackingID
		if result.Status == "" {
			result.Status = firstTracking.Status
			result.Error = firstTracking.Error
		}
	}

	if result.Status == "" {
		result.Status = "accepted"
	}

	return result, nil
}

func filterSuppressedWithStatuses(addrs []string, statuses map[string]port.SuppressionStatus) []string {
	if len(addrs) == 0 {
		return nil
	}

	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if !statuses[addr].Suppressed {
			result = append(result, addr)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
