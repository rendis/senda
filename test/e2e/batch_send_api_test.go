//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestF05A_APIBatchSendPerItemContext verifies POST /api/v1/send/batch with
// per-item variables, tracking IDs, and delivery to Mailpit.
func TestF05A_APIBatchSendPerItemContext(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	mailpit := NewMailpitClient(t)
	mailpit.ClearMessages()

	apiKeyValue := createAPIKey(t, client, "api-batch-v2")

	type batchExpectation struct {
		recipient  string
		name       string
		externalID string
		locale     string
	}

	expectations := []batchExpectation{
		{
			recipient:  "batch-item-ana@test.example.com",
			name:       "Ana",
			externalID: "api-batch-ana",
			locale:     "en",
		},
		{
			recipient:  "batch-item-beto@test.example.com",
			name:       "Beto",
			externalID: "api-batch-beto",
			locale:     "en",
		},
	}

	var responseBody struct {
		Status           string `json:"status"`
		TemplateResolved string `json:"template_resolved"`
		AcceptedCount    int    `json:"accepted_count"`
		SuppressedCount  int    `json:"suppressed_count"`
		FailedCount      int    `json:"failed_count"`
		Items            []struct {
			Index      int    `json:"index"`
			To         string `json:"to"`
			TrackingID string `json:"tracking_id"`
			Status     string `json:"status"`
			ExternalID string `json:"external_id"`
			Error      string `json:"error"`
		} `json:"items"`
	}

	t.Run("POST /send/batch accepts logical messages with isolated variables", func(t *testing.T) {
		sendClient := NewTestClient(t)
		sendClient.SetAPIKey(apiKeyValue)

		req := SendBatchRequest{
			Ref: sendRef(),
			Items: []SendBatchItemRequest{
				{
					To: expectations[0].recipient,
					Variables: map[string]interface{}{
						"first_name":   expectations[0].name,
						"company_name": "Batch Corp",
					},
					ExternalID: expectations[0].externalID,
					Locale:     expectations[0].locale,
				},
				{
					To: expectations[1].recipient,
					Variables: map[string]interface{}{
						"first_name":   expectations[1].name,
						"company_name": "Batch Corp",
					},
					ExternalID: expectations[1].externalID,
					Locale:     expectations[1].locale,
				},
			},
		}

		resp := sendClient.Post("/api/v1/send/batch", req)
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusAccepted)
		ParseJSONResponse(t, resp, &responseBody)

		require.Equal(t, "accepted", responseBody.Status)
		require.Equal(t, sendRef(), responseBody.TemplateResolved)
		require.Equal(t, len(expectations), responseBody.AcceptedCount)
		require.Zero(t, responseBody.SuppressedCount)
		require.Zero(t, responseBody.FailedCount)
		require.Len(t, responseBody.Items, len(expectations))

		for index, item := range responseBody.Items {
			require.Equal(t, index, item.Index)
			require.Equal(t, expectations[index].recipient, item.To)
			require.Equal(t, expectations[index].externalID, item.ExternalID)
			require.Equal(t, "accepted", item.Status)
			require.NotEmpty(t, item.TrackingID)
			require.Empty(t, item.Error)
		}
	})

	t.Run("batch items transition to sent", func(t *testing.T) {
		for _, item := range responseBody.Items {
			client.WaitForEmailStatus(TenantCode, WorkspaceCode, item.TrackingID, "sent", 45*time.Second)
		}
	})

	t.Run("Mailpit receives every batch item with its own rendered variables", func(t *testing.T) {
		mailpit.WaitForMessages(len(expectations), 30*time.Second)

		for _, expected := range expectations {
			msg := mailpit.AssertMessageExists(expected.recipient)
			require.NotNil(t, msg)
			require.Contains(t, msg.Subject, "Welcome")
			require.Contains(t, msg.HTML, expected.name)
		}
	})
}
