//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestE13_HotColdPayloadPersistence(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	mailpit := NewMailpitClient(t)
	mailpit.ClearMessages()

	_, apiKeyValue := MustCreateAPIKey(t, client, TenantCode, WorkspaceCode, "hot-cold-persist")

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKeyValue)

	recipient := fmt.Sprintf("payload-%d@example.com", time.Now().UnixNano())
	sendResp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  []string{recipient},
		Variables: map[string]interface{}{
			"first_name":   "Payload",
			"company_name": "Probe Corp",
		},
	})
	defer sendResp.Body.Close()
	RequireStatus(t, sendResp, http.StatusAccepted)

	var sendBody struct {
		TrackingIDs []struct {
			TrackingID string `json:"tracking_id"`
		} `json:"tracking_ids"`
	}
	ParseJSONResponse(t, sendResp, &sendBody)
	require.Len(t, sendBody.TrackingIDs, 1)

	trackingID := sendBody.TrackingIDs[0].TrackingID
	client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, "sent", 45*time.Second)

	conn := dbConn(t)
	ctx := context.Background()

	var (
		status        string
		payloadBody   string
		payloadVars   string
		payloadExists bool
	)
	err := conn.QueryRow(ctx, `
		SELECT
			e.status,
			ep.body_mjml,
			COALESCE(ep.variables_snapshot::text, '{}'),
			ep.email_id IS NOT NULL
		FROM emails e
		LEFT JOIN email_payloads ep
		  ON ep.email_id = e.id
		 AND ep.email_created_at = e.created_at
		WHERE e.tracking_id = $1
	`, trackingID).Scan(&status, &payloadBody, &payloadVars, &payloadExists)
	require.NoError(t, err, "expected email + cold payload rows for tracking id %s", trackingID)
	require.Equal(t, "sent", status)
	require.True(t, payloadExists, "expected email_payloads row to exist")
	require.Contains(t, payloadBody, "Welcome {{ event.first_name }}!")
	require.True(t, strings.Contains(payloadVars, `"first_name":"Payload"`) || strings.Contains(payloadVars, `"first_name": "Payload"`),
		"expected variables snapshot to contain request payload, got %s", payloadVars)

	mailpit.WaitForMessages(1, 30*time.Second)
	message := mailpit.AssertMessageExists(recipient)
	require.Contains(t, message.HTML, "Payload")
}
