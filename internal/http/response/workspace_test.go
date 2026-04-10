package response_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/response"
)

func TestNewWorkspaceResponse_EmptyRecipientAddressesMarshalAsArray(t *testing.T) {
	ws := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    uuid.New(),
		Code:        "marketing",
		Name:        "Marketing",
		Environment: domain.EnvironmentProd,
		CreatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	}

	body, err := json.Marshal(response.NewWorkspaceResponse(ws))
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	const expected = `"test_recipient_addresses":[]`
	if !strings.Contains(string(body), expected) {
		t.Fatalf("expected marshaled response to contain %s, got %s", expected, string(body))
	}
}
