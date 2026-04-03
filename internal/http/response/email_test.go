package response

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
)

func TestNewEmailResponse_MapsSourceTracking(t *testing.T) {
	memberID := uuid.Must(uuid.NewV7())
	memberEmail := "editor@acme.com"

	email := &domain.Email{
		ID:                  uuid.Must(uuid.NewV7()),
		TrackingID:          "trk_123",
		WorkspaceID:         uuid.Must(uuid.NewV7()),
		TenantID:            uuid.Must(uuid.NewV7()),
		AdapterID:           uuid.Must(uuid.NewV7()),
		TemplateTypeSlug:    "welcome",
		TemplateRef:         "latam:acme:welcome",
		RecipientEmail:      "ana@example.com",
		FromEmail:           "hello@example.com",
		FromName:            "Acme",
		SubjectRendered:     "Hello Ana",
		Status:              domain.StatusQueued,
		SourceType:          domain.EmailSourceTypeManagementTemplateBulkUpload,
		SourceActorMemberID: &memberID,
		SourceActorEmail:    &memberEmail,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	got := NewEmailResponse(email)

	if got.SourceType != string(domain.EmailSourceTypeManagementTemplateBulkUpload) {
		t.Fatalf("expected source type %q, got %q", domain.EmailSourceTypeManagementTemplateBulkUpload, got.SourceType)
	}
	if got.SourceActorMemberID == nil || *got.SourceActorMemberID != memberID.String() {
		t.Fatalf("expected source actor member %s, got %+v", memberID, got.SourceActorMemberID)
	}
	if got.SourceActorEmail == nil || *got.SourceActorEmail != memberEmail {
		t.Fatalf("expected source actor email %q, got %+v", memberEmail, got.SourceActorEmail)
	}
}
