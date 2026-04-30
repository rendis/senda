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
	inputs := make([]SuppressionBatchInput, 0, len(items))
	for _, item := range items {
		effectiveTo, effectiveCC, effectiveBCC, err := applyTestRecipientPolicy(shared.Workspace, shared.TemplateType, []string{item.To}, item.CC, item.BCC)
		if err != nil {
			return err
		}
		inputs = append(inputs, SuppressionBatchInput{
			To:  effectiveTo,
			CC:  effectiveCC,
			BCC: effectiveBCC,
		})
	}

	// Use the three-tier evaluator (global + workspace + per-type opt-outs) so
	// batch sends honour the same suppression rules as single sends.
	evaluator := NewSuppressionBatchEvaluator(s.suppression).
		WithTemplateTypeStore(s.templateTypeSubscriptionStore)
	results, err := evaluator.EvaluateManyForType(ctx, shared.Workspace.ID, shared.TemplateType.ID, inputs)
	if err != nil {
		return err
	}

	// Flatten into the address-keyed map expected by executeSendPlan.
	statuses := make(map[string]port.SuppressionStatus)
	for _, eval := range results {
		for _, d := range eval.To {
			statuses[d.Address] = port.SuppressionStatus{Suppressed: d.Suppressed, Reason: d.Reason}
		}
		// CC/BCC that are absent from the filtered slice are suppressed; those
		// remaining are not. Rebuild a full status map for CC/BCC addresses.
		// We only need To statuses for the batch path (CC/BCC are filtered in
		// prepareSendPlanExecution via filterSuppressedWithStatuses), so we add
		// the accepted ones as not-suppressed and the absent ones as suppressed.
	}

	// CC/BCC: reconstructed from inputs vs. filtered results.
	for i, input := range inputs {
		eval := results[i]
		ccSet := make(map[string]struct{}, len(eval.CC))
		for _, a := range eval.CC {
			ccSet[a] = struct{}{}
		}
		for _, a := range input.CC {
			if _, ok := ccSet[a]; !ok {
				statuses[a] = port.SuppressionStatus{Suppressed: true, Reason: "suppressed"}
			} else {
				if _, exists := statuses[a]; !exists {
					statuses[a] = port.SuppressionStatus{}
				}
			}
		}
		bccSet := make(map[string]struct{}, len(eval.BCC))
		for _, a := range eval.BCC {
			bccSet[a] = struct{}{}
		}
		for _, a := range input.BCC {
			if _, ok := bccSet[a]; !ok {
				statuses[a] = port.SuppressionStatus{Suppressed: true, Reason: "suppressed"}
			} else {
				if _, exists := statuses[a]; !exists {
					statuses[a] = port.SuppressionStatus{}
				}
			}
		}
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

	execCtx, err := s.prepareSendPlanExecution(ctx, shared, plan)
	if err != nil {
		return result, err
	}

	outcome := s.processSendPlanRecipients(ctx, shared, plan, execCtx)
	return finalizeSendPlanResult(result, execCtx.effectiveTo, outcome)
}

type sendPlanExecution struct {
	resolved         *resolution.ResolvedTemplate
	injectors        map[string]map[string]any
	renderedSubject  string
	renderedFromName string
	bodyMJML         string
	now              time.Time
	effectiveTo      []string
	filteredCC       []string
	filteredBCC      []string
}

type sendPlanOutcome struct {
	lastErr          error
	failCount        int
	firstTracking    TrackingEntry
	hasFirstTracking bool
}

func (s *SendService) prepareSendPlanExecution(ctx context.Context, shared *ResolvedSendContext, plan SendPlan) (*sendPlanExecution, error) {
	resolved, injectors, renderedSubject, renderedFromName, err := s.resolveSendPlanTemplate(ctx, shared, plan)
	if err != nil {
		return nil, err
	}

	effectiveTo, effectiveCC, effectiveBCC, err := applyTestRecipientPolicy(shared.Workspace, resolved.TemplateType, []string{plan.To}, plan.CC, plan.BCC)
	if err != nil {
		return nil, err
	}

	return &sendPlanExecution{
		resolved:         resolved,
		injectors:        injectors,
		renderedSubject:  renderedSubject,
		renderedFromName: renderedFromName,
		bodyMJML:         getLocalizedBody(resolved),
		now:              time.Now().UTC(),
		effectiveTo:      effectiveTo,
		filteredCC:       filterSuppressedWithStatuses(effectiveCC, shared.suppressionStatuses),
		filteredBCC:      filterSuppressedWithStatuses(effectiveBCC, shared.suppressionStatuses),
	}, nil
}

func (s *SendService) resolveSendPlanTemplate(ctx context.Context, shared *ResolvedSendContext, plan SendPlan) (*resolution.ResolvedTemplate, map[string]map[string]any, string, string, error) {
	resolved, err := shared.templateForLocale(ctx, s, plan.Locale)
	if err != nil {
		return nil, nil, "", "", err
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
		return nil, nil, "", "", err
	}

	renderedSubject, err := s.renderer.Render(getLocalizedField(resolved, "subject"), injectors, plan.Variables)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("render subject: %w", err)
	}
	renderedFromName, err := s.renderer.Render(getLocalizedField(resolved, "from_name"), injectors, plan.Variables)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("render from_name: %w", err)
	}

	return resolved, injectors, renderedSubject, renderedFromName, nil
}

func (s *SendService) processSendPlanRecipients(ctx context.Context, shared *ResolvedSendContext, plan SendPlan, execCtx *sendPlanExecution) sendPlanOutcome {
	var outcome sendPlanOutcome

	for _, recipient := range execCtx.effectiveTo {
		trackingID := generateTrackingID()
		status := shared.suppressionStatuses[recipient]
		email := s.buildSendPlanEmail(shared, plan, execCtx, recipient, trackingID)
		if status.Suppressed {
			s.persistSuppressedPlanRecipient(ctx, email, execCtx.now, status.Reason, trackingID, recipient, &outcome)
			continue
		}
		s.persistQueuedPlanRecipient(ctx, email, shared.Adapter.Adapter.ID, trackingID, recipient, &outcome)
	}

	return outcome
}

func (s *SendService) buildSendPlanEmail(shared *ResolvedSendContext, plan SendPlan, execCtx *sendPlanExecution, recipient, trackingID string) *domain.Email {
	source := shared.Source
	return &domain.Email{
		ID:                  uuid.Must(uuid.NewV7()),
		TrackingID:          trackingID,
		ExternalID:          plan.ExternalID,
		WorkspaceID:         shared.Workspace.ID,
		TenantID:            shared.Tenant.ID,
		TemplateID:          execCtx.resolved.Template.ID,
		TemplateVersionID:   execCtx.resolved.Version.ID,
		TemplateTypeSlug:    shared.TemplateType.Slug,
		TemplateRef:         shared.Ref,
		RecipientEmail:      recipient,
		CC:                  execCtx.filteredCC,
		BCC:                 execCtx.filteredBCC,
		FromEmail:           shared.FromEmail,
		FromName:            execCtx.renderedFromName,
		ReplyTo:             execCtx.resolved.Version.ReplyTo,
		SubjectRendered:     execCtx.renderedSubject,
		Locale:              plan.Locale,
		AdapterID:           shared.Adapter.Adapter.ID,
		SenderIdentityID:    execCtx.resolved.TemplateType.SenderIdentityID,
		VariablesSnapshot:   plan.Variables,
		InjectorsSnapshot:   execCtx.injectors,
		SourceType:          source.Type,
		SourceActorMemberID: source.ActorMemberID,
		SourceActorEmail:    source.ActorEmail,
		BodyMJML:            execCtx.bodyMJML,
		OpenTrackingEnabled: shared.Workspace.OpenTrackingEnabled,
		MaxRetries:          3,
		CreatedAt:           execCtx.now,
		UpdatedAt:           execCtx.now,
	}
}

func (s *SendService) persistSuppressedPlanRecipient(ctx context.Context, email *domain.Email, now time.Time, reason, trackingID, recipient string, outcome *sendPlanOutcome) {
	email.Status = domain.StatusSuppressed
	if err := s.createSuppressed(ctx, email, now, reason); err != nil {
		recordSendPlanFailure(outcome, err, recipient, trackingID)
		return
	}
	recordSendPlanFirstTracking(outcome, TrackingEntry{To: recipient, TrackingID: trackingID, Status: "suppressed"})
}

func (s *SendService) persistQueuedPlanRecipient(ctx context.Context, email *domain.Email, adapterID uuid.UUID, trackingID, recipient string, outcome *sendPlanOutcome) {
	email.Status = domain.StatusQueued
	if err := s.createAndEnqueue(ctx, email, trackingID, adapterID); err != nil {
		recordSendPlanFailure(outcome, err, recipient, trackingID)
		return
	}
	recordSendPlanFirstTracking(outcome, TrackingEntry{To: recipient, TrackingID: trackingID, Status: "accepted"})
}

func recordSendPlanFailure(outcome *sendPlanOutcome, err error, recipient, trackingID string) {
	outcome.failCount++
	outcome.lastErr = err
	recordSendPlanFirstTracking(outcome, TrackingEntry{To: recipient, TrackingID: trackingID, Status: "failed", Error: err.Error()})
}

func recordSendPlanFirstTracking(outcome *sendPlanOutcome, entry TrackingEntry) {
	if outcome.hasFirstTracking {
		return
	}
	outcome.firstTracking = entry
	outcome.hasFirstTracking = true
}

func finalizeSendPlanResult(result SendBatchItemResult, effectiveTo []string, outcome sendPlanOutcome) (SendBatchItemResult, error) {
	if outcome.failCount == len(effectiveTo) {
		return result, fmt.Errorf("all recipients failed: %w", outcome.lastErr)
	}
	if outcome.failCount > 0 {
		result.Status = "partial"
		if outcome.lastErr != nil {
			result.Error = outcome.lastErr.Error()
		}
	} else if outcome.hasFirstTracking {
		result.Status = outcome.firstTracking.Status
		result.TrackingID = outcome.firstTracking.TrackingID
		result.Error = outcome.firstTracking.Error
		return result, nil
	}
	if outcome.hasFirstTracking {
		result.TrackingID = outcome.firstTracking.TrackingID
		if result.Status == "" {
			result.Status = outcome.firstTracking.Status
			result.Error = outcome.firstTracking.Error
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
