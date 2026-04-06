//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestF06_AdapterTestSend verifies the adapter test-send flow with
// optional from-address selection and disabled-state when no identities exist.
func TestF06_AdapterTestSend(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wp := wsPath()

	// ── Create a fresh adapter for isolation ──
	adapterName := fmt.Sprintf("test-send-adapter-%d", time.Now().UnixNano())
	createResp := client.Post(wp+"/adapters", AdapterRequest{
		Name:        adapterName,
		AdapterType: AdapterType,
		Config: map[string]interface{}{
			"region":            "us-east-1",
			"access_key_id":     "test",
			"secret_access_key": "test",
		},
		RateLimitPerSecond: 100,
	})
	defer createResp.Body.Close()
	RequireStatus(t, createResp, http.StatusCreated)

	var adapterBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, createResp, &adapterBody)
	adapterID := adapterBody.ID
	require.NotEmpty(t, adapterID)

	t.Run("test send without identities returns NO_DEFAULT_IDENTITY", func(t *testing.T) {
		resp := client.Post(wp+"/adapters/"+adapterID+"/test", map[string]any{
			"to":      "test@example.com",
			"subject": "Test",
			"body":    "<p>Test</p>",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusUnprocessableEntity)

		errResp := ParseError(t, resp)
		require.Equal(t, "NO_DEFAULT_IDENTITY", errResp.Error.Code)
	})

	t.Run("list identities returns empty for fresh adapter", func(t *testing.T) {
		resp := client.Get(wp + "/adapters/" + adapterID + "/identities")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var identities []struct {
			ID           string `json:"id"`
			Identity     string `json:"identity"`
			IdentityType string `json:"identity_type"`
			Status       string `json:"status"`
		}
		ParseJSONResponse(t, resp, &identities)
		require.Empty(t, identities, "fresh adapter should have no identities")
	})

	// ── Seed verified email identities via DB ──
	fromEmail1 := "sender1@mail.test.example.com"
	fromEmail2 := "sender2@mail.test.example.com"
	EnsureAdapterIdentity(t, adapterID, fromEmail1, true)
	EnsureAdapterIdentity(t, adapterID, fromEmail2, false)

	t.Run("list identities returns seeded emails", func(t *testing.T) {
		resp := client.Get(wp + "/adapters/" + adapterID + "/identities")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var identities []struct {
			ID           string `json:"id"`
			Identity     string `json:"identity"`
			IdentityType string `json:"identity_type"`
			Status       string `json:"status"`
			IsDefault    bool   `json:"is_default"`
		}
		ParseJSONResponse(t, resp, &identities)

		verifiedEmails := 0
		for _, id := range identities {
			if id.IdentityType == "email" && id.Status == "verified" {
				verifiedEmails++
			}
		}
		require.GreaterOrEqual(t, verifiedEmails, 2, "should have at least 2 verified email identities")
	})

	// Note: actual SES delivery will fail (no AWS mock in core e2e stack).
	// The test verifies that identity resolution passes before the send attempt.
	// A SEND_FAILED error means identity selection worked but SES rejected the call.

	t.Run("test send with explicit from passes identity resolution", func(t *testing.T) {
		resp := client.Post(wp+"/adapters/"+adapterID+"/test", map[string]any{
			"to":      "test@example.com",
			"subject": "Test from sender2",
			"body":    "<p>Test with from selection</p>",
			"from":    fromEmail2,
		})
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var body struct {
				From string `json:"from"`
			}
			ParseJSONResponse(t, resp, &body)
			require.Equal(t, fromEmail2, body.From)
		} else {
			RequireStatus(t, resp, http.StatusUnprocessableEntity)
			errResp := ParseError(t, resp)
			require.Equal(t, "SEND_FAILED", errResp.Error.Code,
				"expected SEND_FAILED (identity resolved, delivery failed), got %s: %s",
				errResp.Error.Code, errResp.Error.Message)
		}
	})

	t.Run("test send with default from passes identity resolution", func(t *testing.T) {
		resp := client.Post(wp+"/adapters/"+adapterID+"/test", map[string]any{
			"to":      "test@example.com",
			"subject": "Test default from",
			"body":    "<p>Test without from selection</p>",
		})
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var body struct {
				From string `json:"from"`
			}
			ParseJSONResponse(t, resp, &body)
			require.Equal(t, fromEmail1, body.From, "should use default identity")
		} else {
			RequireStatus(t, resp, http.StatusUnprocessableEntity)
			errResp := ParseError(t, resp)
			require.Equal(t, "SEND_FAILED", errResp.Error.Code,
				"expected SEND_FAILED (default identity resolved, delivery failed), got %s: %s",
				errResp.Error.Code, errResp.Error.Message)
		}
	})

	t.Run("test send with invalid from returns INVALID_FROM", func(t *testing.T) {
		resp := client.Post(wp+"/adapters/"+adapterID+"/test", map[string]any{
			"to":      "test@example.com",
			"subject": "Test invalid from",
			"body":    "<p>Test with invalid from</p>",
			"from":    "nonexistent@example.com",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusUnprocessableEntity)

		errResp := ParseError(t, resp)
		require.Equal(t, "INVALID_FROM", errResp.Error.Code)
	})

	t.Run("test send with from matching domain identity returns INVALID_FROM", func(t *testing.T) {
		resp := client.Post(wp+"/adapters/"+adapterID+"/test", map[string]any{
			"to":      "test@example.com",
			"subject": "Test domain from",
			"body":    "<p>Test with domain from</p>",
			"from":    "mail.test.example.com",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusUnprocessableEntity)

		errResp := ParseError(t, resp)
		require.Equal(t, "INVALID_FROM", errResp.Error.Code)
	})
}
