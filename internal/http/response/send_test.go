package response

import (
	"encoding/json"
	"testing"

	"github.com/rendis/senda/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSendEmailResponse_MapsPerRecipientStatusAndError(t *testing.T) {
	extID := "ext-123"
	svcResp := &service.SendResponse{
		Status:           "partial",
		TemplateResolved: "latam:acme:welcome",
		TemplateVersion:  3,
		ExternalID:       &extID,
		TrackingIDs: []service.TrackingEntry{
			{To: "ok@example.com", TrackingID: "trk_aaa", Status: "accepted"},
			{To: "suppressed@example.com", TrackingID: "trk_bbb", Status: "suppressed"},
			{To: "fail@example.com", TrackingID: "trk_ccc", Status: "failed", Error: "create email: db error"},
		},
	}

	got := NewSendEmailResponse(svcResp)

	assert.Equal(t, "partial", got.Status)
	assert.Equal(t, "latam:acme:welcome", got.TemplateResolved)
	assert.Equal(t, 3, got.TemplateVersion)
	require.NotNil(t, got.ExternalID)
	assert.Equal(t, "ext-123", *got.ExternalID)

	require.Len(t, got.TrackingIDs, 3)

	// First entry: accepted — no error field.
	assert.Equal(t, "ok@example.com", got.TrackingIDs[0].To)
	assert.Equal(t, "trk_aaa", got.TrackingIDs[0].TrackingID)
	assert.Equal(t, "accepted", got.TrackingIDs[0].Status)
	assert.Empty(t, got.TrackingIDs[0].Error)

	// Second entry: suppressed.
	assert.Equal(t, "suppressed@example.com", got.TrackingIDs[1].To)
	assert.Equal(t, "suppressed", got.TrackingIDs[1].Status)
	assert.Empty(t, got.TrackingIDs[1].Error)

	// Third entry: failed — must carry error.
	assert.Equal(t, "fail@example.com", got.TrackingIDs[2].To)
	assert.Equal(t, "trk_ccc", got.TrackingIDs[2].TrackingID)
	assert.Equal(t, "failed", got.TrackingIDs[2].Status)
	assert.Equal(t, "create email: db error", got.TrackingIDs[2].Error)
}

func TestNewSendEmailResponse_JSONOmitsEmptyError(t *testing.T) {
	svcResp := &service.SendResponse{
		Status:           "accepted",
		TemplateResolved: "latam:acme:welcome",
		TemplateVersion:  1,
		TrackingIDs: []service.TrackingEntry{
			{To: "ok@example.com", TrackingID: "trk_111", Status: "accepted"},
			{To: "bad@example.com", TrackingID: "trk_222", Status: "failed", Error: "timeout"},
		},
	}

	got := NewSendEmailResponse(svcResp)
	data, err := json.Marshal(got)
	require.NoError(t, err)

	// accepted entry must NOT have "error" key in JSON.
	var raw struct {
		TrackingIDs []map[string]any `json:"tracking_ids"`
	}
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Len(t, raw.TrackingIDs, 2)

	_, hasError := raw.TrackingIDs[0]["error"]
	assert.False(t, hasError, "accepted entry should not serialize 'error' field")

	_, hasError = raw.TrackingIDs[1]["error"]
	assert.True(t, hasError, "failed entry must serialize 'error' field")
	assert.Equal(t, "timeout", raw.TrackingIDs[1]["error"])
}
