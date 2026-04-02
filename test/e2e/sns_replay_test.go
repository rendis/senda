//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rendis/senda/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSNSReplay01_SignedNotificationsRemainSupported(t *testing.T) {
	EnsureSetup(t)
	replay := startReplayHarness(t)
	t.Cleanup(replay.Close)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	cases := []struct {
		name           string
		notification   string
		expectedStatus string
		recipient      string
	}{
		{name: "delivery", notification: "Delivery", expectedStatus: string(domain.StatusDelivered), recipient: "signed-replay-delivery@test.example.com"},
		{name: "bounce", notification: "Bounce", expectedStatus: string(domain.StatusBounced), recipient: "signed-replay-bounce@test.example.com"},
		{name: "complaint", notification: "Complaint", expectedStatus: string(domain.StatusComplained), recipient: "signed-replay-complaint@test.example.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trackingID := sendThroughSMTPBaseline(t, client, tc.recipient, "sns-replay-"+tc.name)
			client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusSent), 45*time.Second)

			providerMessageID := smtpProviderMessageID(trackingID)
			notification := buildSESNotification(t, tc.notification, providerMessageID, tc.recipient, time.Now().UTC())
			envelope := snsEnvelope{
				Type:      "Notification",
				MessageID: uniqueName("sns-msg"),
				TopicArn:  "arn:aws:sns:us-east-1:123456789012:senda-ses-events",
				Message:   string(notification),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			signed := signSNSMessage(t, envelope, replay.certURL, replay.privateKey)

			resp, err := replay.POST("/api/v1/webhooks/ses/inbound", signed)
			require.NoError(t, err)
			defer resp.Body.Close()
			RequireStatus(t, resp, http.StatusOK)

			client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, tc.expectedStatus, 45*time.Second)
		})
	}
}

func sendThroughSMTPBaseline(t *testing.T, client *TestClient, recipient, suffix string) string {
	t.Helper()

	apiKeyValue := createAPIKey(t, client, suffix)
	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKeyValue)

	resp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: fmt.Sprintf("%s:%s:%s", TenantCode, WorkspaceCode, TemplateTypeSlug),
		To:  []string{recipient},
		Variables: map[string]interface{}{
			"first_name":   "Replay",
			"company_name": "Senda",
		},
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusAccepted)

	var body struct {
		TrackingIDs []struct {
			TrackingID string `json:"tracking_id"`
		} `json:"tracking_ids"`
	}
	ParseJSONResponse(t, resp, &body)
	require.Len(t, body.TrackingIDs, 1)
	return body.TrackingIDs[0].TrackingID
}
