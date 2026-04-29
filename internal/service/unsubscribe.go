package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/unsubscribe"
	"github.com/rendis/senda/pkg/apperr"
)

// Public DTO types (consumed by HTTP layer in Task 12).

// UnsubscribeContext is returned by GetContext for the unsubscribe landing page.
type UnsubscribeContext struct {
	WorkspaceName    string
	TemplateTypeSlug string
	TemplateTypeName string
	Email            string
	OptedOutOfType   bool
	OptedOutOfAll    bool
}

// PreferencesEntry is one row in the preference center view.
type PreferencesEntry struct {
	TemplateTypeSlug string
	TemplateTypeName string
	Description      *string
	Subscribed       bool
	LastReceivedAt   time.Time
}

// PreferencesView is returned by GetPreferences.
type PreferencesView struct {
	WorkspaceName string
	Email         string
	OptedOutOfAll bool
	Entries       []PreferencesEntry
}

// PreferenceChange is one toggle submitted by the preference center form.
type PreferenceChange struct {
	TemplateTypeSlug string
	Subscribed       bool
}

// EmailHistoryType is the result of "which template types did this recipient receive in the last N months".
type EmailHistoryType struct {
	Slug       string
	LastSentAt time.Time
}

// ErrInvalidToken is returned when a token cannot be verified (malformed, wrong
// key, expired, or pointing to a workspace that does not exist). Callers may
// use errors.Is(err, ErrInvalidToken) to map this to an HTTP 400/401.
var ErrInvalidToken = errors.New("unsubscribe: invalid token")

// Local interfaces — narrowest dependencies for hexagonal cleanliness.

type unsubWorkspaceLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	GetUnsubscribeSigningKey(ctx context.Context, workspaceID uuid.UUID) ([]byte, error)
}

type unsubTemplateTypeLookup interface {
	FindTypeBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error)
}

// unsubSuppressionWS operates on workspace-level unsubscribe suppression rows.
// Satisfied by port.SuppressionStore / postgres.SuppressionRepo.
type unsubSuppressionWS interface {
	// AddWorkspace inserts a new suppression_workspace row.
	AddWorkspace(ctx context.Context, sup *domain.SuppressionWorkspace) error
	// GetActiveWorkspaceSuppression returns the currently-active (removed_at IS NULL) row for
	// (workspace, email), or an apperr.NotFound error when none exists.
	GetActiveWorkspaceSuppression(ctx context.Context, workspaceID uuid.UUID, email string) (*domain.SuppressionWorkspace, error)
	// RemoveWorkspaceSuppression sets removed_at and removal_reason on the active row.
	RemoveWorkspaceSuppression(ctx context.Context, workspaceID uuid.UUID, email string, reason string) error
}

type unsubTTSWriter interface {
	Upsert(ctx context.Context, sub *domain.TemplateTypeSubscription) error
	GetState(ctx context.Context, workspaceID, templateTypeID uuid.UUID, email string) (*domain.TemplateTypeSubscription, error)
	ListOptOutsForRecipient(ctx context.Context, workspaceID uuid.UUID, email string) ([]*domain.TemplateTypeSubscription, error)
}

type unsubEmailHistory interface {
	DistinctTemplateTypesForRecipient(ctx context.Context, workspaceID uuid.UUID, email string, since time.Time) ([]EmailHistoryType, error)
}

// UnsubscribeService implements the public unsubscribe operations.
type UnsubscribeService struct {
	ws      unsubWorkspaceLookup
	tt      unsubTemplateTypeLookup
	supWS   unsubSuppressionWS
	tts     unsubTTSWriter
	history unsubEmailHistory
	now     func() time.Time
}

// NewUnsubscribeService constructs an UnsubscribeService with the required
// dependencies. The history dependency is satisfied by a separate store added
// in Task 11; pass nil until then (or provide a stub in tests).
func NewUnsubscribeService(
	ws unsubWorkspaceLookup,
	tt unsubTemplateTypeLookup,
	supWS unsubSuppressionWS,
	tts unsubTTSWriter,
	history unsubEmailHistory,
) *UnsubscribeService {
	return &UnsubscribeService{
		ws: ws, tt: tt, supWS: supWS, tts: tts, history: history,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// verify peeks the workspace ID from the token (without HMAC), fetches the
// per-workspace signing key, then performs a full constant-time HMAC verify.
func (s *UnsubscribeService) verify(ctx context.Context, token string) (unsubscribe.Payload, error) {
	wsID, ok := unsubscribe.PeekWorkspaceID(token)
	if !ok {
		return unsubscribe.Payload{}, ErrInvalidToken
	}
	key, err := s.ws.GetUnsubscribeSigningKey(ctx, wsID)
	if err != nil {
		return unsubscribe.Payload{}, fmt.Errorf("%w: workspace lookup: %w", ErrInvalidToken, err)
	}
	p, err := unsubscribe.Verify(token, key, s.now())
	if err != nil {
		return unsubscribe.Payload{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return p, nil
}

// GetContext returns the data needed to render the unsubscribe landing page.
func (s *UnsubscribeService) GetContext(ctx context.Context, token string) (*UnsubscribeContext, error) {
	p, err := s.verify(ctx, token)
	if err != nil {
		return nil, err
	}

	ws, err := s.ws.GetByID(ctx, p.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("workspace name: %w", err)
	}

	optedOutType := false
	tt, err := s.tt.FindTypeBySlugInScope(ctx, p.TemplateTypeSlug, &p.WorkspaceID)
	if err != nil && !apperr.IsNotFound(err) {
		return nil, err
	}
	if tt != nil {
		sub, err := s.tts.GetState(ctx, p.WorkspaceID, tt.ID, p.Email)
		if err != nil && !apperr.IsNotFound(err) {
			return nil, err
		}
		if sub != nil && !sub.Subscribed {
			optedOutType = true
		}
	}

	sup, err := s.supWS.GetActiveWorkspaceSuppression(ctx, p.WorkspaceID, p.Email)
	if err != nil && !apperr.IsNotFound(err) {
		return nil, err
	}
	optedAll := sup != nil && sup.Reason == domain.SuppressionUnsubscribe

	return &UnsubscribeContext{
		WorkspaceName:    ws.Name,
		TemplateTypeSlug: p.TemplateTypeSlug,
		TemplateTypeName: p.TemplateTypeName,
		Email:            p.Email,
		OptedOutOfType:   optedOutType,
		OptedOutOfAll:    optedAll,
	}, nil
}

// OneClickOptOut handles RFC 8058 one-click POST — opts the recipient out of
// the specific template type embedded in the token.
func (s *UnsubscribeService) OneClickOptOut(ctx context.Context, token string) error {
	p, err := s.verify(ctx, token)
	if err != nil {
		return err
	}
	tt, err := s.tt.FindTypeBySlugInScope(ctx, p.TemplateTypeSlug, &p.WorkspaceID)
	if err != nil {
		return fmt.Errorf("resolve type %s: %w", p.TemplateTypeSlug, err)
	}
	if tt == nil {
		return fmt.Errorf("%w: template type %s not found", ErrInvalidToken, p.TemplateTypeSlug)
	}
	sourceID := p.SourceEmailID
	return s.tts.Upsert(ctx, &domain.TemplateTypeSubscription{
		ID:             uuid.Must(uuid.NewV7()),
		WorkspaceID:    p.WorkspaceID,
		TemplateTypeID: tt.ID,
		Email:          p.Email,
		Subscribed:     false,
		Source:         domain.SubscriptionSourceRecipientOptout,
		SourceEmailID:  &sourceID,
	})
}

// OptOutAll adds a workspace-level suppression for the recipient. All future
// emails from this workspace to this address will be blocked regardless of type.
func (s *UnsubscribeService) OptOutAll(ctx context.Context, token string) error {
	p, err := s.verify(ctx, token)
	if err != nil {
		return err
	}
	sourceID := p.SourceEmailID
	return s.supWS.AddWorkspace(ctx, &domain.SuppressionWorkspace{
		ID:            uuid.Must(uuid.NewV7()),
		WorkspaceID:   p.WorkspaceID,
		Email:         p.Email,
		Reason:        domain.SuppressionUnsubscribe,
		SourceEmailID: &sourceID,
	})
}

// Resubscribe removes the workspace-level suppression, re-enabling delivery.
func (s *UnsubscribeService) Resubscribe(ctx context.Context, token string) error {
	p, err := s.verify(ctx, token)
	if err != nil {
		return err
	}
	return s.supWS.RemoveWorkspaceSuppression(ctx, p.WorkspaceID, p.Email, "recipient_resubscribe")
}

// GetPreferences returns the preference center view, populated with every
// template type the recipient received in the last 12 months.
func (s *UnsubscribeService) GetPreferences(ctx context.Context, token string) (*PreferencesView, error) {
	p, err := s.verify(ctx, token)
	if err != nil {
		return nil, err
	}

	ws, err := s.ws.GetByID(ctx, p.WorkspaceID)
	if err != nil {
		return nil, err
	}

	since := s.now().AddDate(-1, 0, 0)
	history, err := s.history.DistinctTemplateTypesForRecipient(ctx, p.WorkspaceID, p.Email, since)
	if err != nil {
		return nil, err
	}

	sup, err := s.supWS.GetActiveWorkspaceSuppression(ctx, p.WorkspaceID, p.Email)
	if err != nil && !apperr.IsNotFound(err) {
		return nil, err
	}
	optedAll := sup != nil && sup.Reason == domain.SuppressionUnsubscribe

	entries := make([]PreferencesEntry, 0, len(history))
	for _, h := range history {
		tt, err := s.tt.FindTypeBySlugInScope(ctx, h.Slug, &p.WorkspaceID)
		if err != nil || tt == nil {
			continue
		}
		sub, err := s.tts.GetState(ctx, p.WorkspaceID, tt.ID, p.Email)
		subscribed := true
		if err == nil && sub != nil {
			subscribed = sub.Subscribed
		} else if err != nil && !apperr.IsNotFound(err) {
			return nil, err
		}
		entries = append(entries, PreferencesEntry{
			TemplateTypeSlug: tt.Slug,
			TemplateTypeName: tt.Name,
			Description:      tt.Description,
			Subscribed:       subscribed,
			LastReceivedAt:   h.LastSentAt,
		})
	}

	return &PreferencesView{
		WorkspaceName: ws.Name,
		Email:         p.Email,
		OptedOutOfAll: optedAll,
		Entries:       entries,
	}, nil
}

// UpdatePreferences applies one or more opt-in/opt-out changes from the
// preference center form.
func (s *UnsubscribeService) UpdatePreferences(ctx context.Context, token string, changes []PreferenceChange) error {
	p, err := s.verify(ctx, token)
	if err != nil {
		return err
	}
	sourceID := p.SourceEmailID
	for _, c := range changes {
		tt, err := s.tt.FindTypeBySlugInScope(ctx, c.TemplateTypeSlug, &p.WorkspaceID)
		if err != nil {
			return fmt.Errorf("resolve type %s: %w", c.TemplateTypeSlug, err)
		}
		if tt == nil {
			return fmt.Errorf("%w: template type %s not found", ErrInvalidToken, c.TemplateTypeSlug)
		}
		src := domain.SubscriptionSourceRecipientOptin
		if !c.Subscribed {
			src = domain.SubscriptionSourceRecipientOptout
		}
		if err := s.tts.Upsert(ctx, &domain.TemplateTypeSubscription{
			ID:             uuid.Must(uuid.NewV7()),
			WorkspaceID:    p.WorkspaceID,
			TemplateTypeID: tt.ID,
			Email:          p.Email,
			Subscribed:     c.Subscribed,
			Source:         src,
			SourceEmailID:  &sourceID,
		}); err != nil {
			return err
		}
	}
	return nil
}
