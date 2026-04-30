//go:build e2e_local

// To run this test:
//  1. In one terminal:  make dev    (starts docker stack: postgres, mailpit, app, keycloak)
//     Wait for the app to print its listening address.
//  2. In another terminal, start the Next.js frontend:
//     pnpm --dir web dev
//     Wait for "Ready on http://localhost:3000".
//  3. In a third terminal:
//     export SENDA_E2E_LOCAL_API_KEY=<workspace api key from /api/v1/manage/.../api-keys>
//     export SENDA_E2E_LOCAL_BACKEND=http://localhost:8081   # default
//     export SENDA_E2E_LOCAL_FRONTEND=http://localhost:3000  # default
//     export SENDA_E2E_LOCAL_MAILPIT=http://localhost:8026   # default
//     go test -tags=e2e_local -v -timeout 300s ./test/e2e -run TestUnsubscribeUI
//
// The test:
//   - Creates a bulk template_type and template via the backend API.
//   - Sends an email to a unique address.
//   - Polls Mailpit for the message and extracts the unsubscribe token.
//   - Drives Next.js /u/{token} with chromedp: asserts card, clicks "this event", confirms.
//   - Drives /u/{token}/preferences: toggles the type checkbox off, saves.
//   - Asserts a follow-up email is blocked.
//   - Toggles the type checkbox back on, saves.
//   - Asserts delivery resumes.

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"
)

// localEnvOr returns the value of the env var k, or def if unset.
func localEnvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// localPing performs a GET and returns an error if the endpoint is unreachable
// or returns an HTTP 5xx status.
func localPing(url string) error {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("ping %s: status %d", url, resp.StatusCode)
	}
	return nil
}

// localMailpitMsg is a trimmed Mailpit message summary.
type localMailpitMsg struct {
	ID string `json:"ID"`
	To []struct {
		Address string `json:"Address"`
	} `json:"To"`
}

type localMailpitListing struct {
	Messages []localMailpitMsg `json:"messages"`
}

func localClearMailpit(t *testing.T, base string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/messages", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
}

func localGetMessages(t *testing.T, base string) []localMailpitMsg {
	t.Helper()
	resp, err := http.Get(base + "/api/v1/messages") //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var l localMailpitListing
	require.NoError(t, json.Unmarshal(body, &l))
	return l.Messages
}

func localFilterByRecipient(msgs []localMailpitMsg, recipient string) []localMailpitMsg {
	var out []localMailpitMsg
	for _, m := range msgs {
		for _, addr := range m.To {
			if strings.EqualFold(addr.Address, recipient) {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// localWaitForToken polls Mailpit until a message for recipient arrives, then
// extracts the unsubscribe token from the List-Unsubscribe header.
func localWaitForToken(t *testing.T, mailpitBase, recipient string, timeout time.Duration) string {
	t.Helper()
	type fullMsg struct {
		Headers map[string][]string `json:"Headers"`
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msgs := localFilterByRecipient(localGetMessages(t, mailpitBase), recipient)
		if len(msgs) > 0 {
			resp, err := http.Get(mailpitBase + "/api/v1/message/" + msgs[0].ID) //nolint:noctx
			require.NoError(t, err)
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var full fullMsg
			require.NoError(t, json.Unmarshal(body, &full))
			for k, vs := range full.Headers {
				if !strings.EqualFold(k, "List-Unsubscribe") {
					continue
				}
				for _, v := range vs {
					raw := strings.TrimSpace(v)
					raw = strings.TrimPrefix(raw, "<")
					raw = strings.TrimSuffix(raw, ">")
					if idx := strings.Index(raw, "/api/v1/u/"); idx >= 0 {
						tok := raw[idx+len("/api/v1/u/"):]
						tok = strings.SplitN(tok, "/", 2)[0]
						tok = strings.TrimSpace(tok)
						if tok != "" {
							return tok
						}
					}
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("localWaitForToken: token never appeared for %s within %s", recipient, timeout)
	return ""
}

// localAPICall is a minimal helper for authenticated backend API calls using an API key.
func localAPICall(t *testing.T, method, url, apiKey string, body any) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reqBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// TestUnsubscribeUI_FullFlow is a local-only chromedp test that drives the
// Next.js unsubscribe UI end-to-end. Run with:
//
//	make test-ui-unsubscribe
func TestUnsubscribeUI_FullFlow(t *testing.T) {
	backend := localEnvOr("SENDA_E2E_LOCAL_BACKEND", "http://localhost:8081")
	frontend := localEnvOr("SENDA_E2E_LOCAL_FRONTEND", "http://localhost:3000")
	mailpit := localEnvOr("SENDA_E2E_LOCAL_MAILPIT", "http://localhost:8026")
	apiKey := os.Getenv("SENDA_E2E_LOCAL_API_KEY")
	require.NotEmpty(t, apiKey, "set SENDA_E2E_LOCAL_API_KEY to a workspace API key (senda_prod_... or senda_test_...)")

	// Sanity: all three services reachable.
	require.NoError(t, localPing(backend+"/healthz"), "backend not reachable at %s", backend)
	require.NoError(t, localPing(mailpit+"/api/v1/messages"), "mailpit not reachable at %s", mailpit)
	require.NoError(t, localPing(frontend+"/"), "frontend not reachable at %s (run `pnpm --dir web dev`)", frontend)

	// Clear mailpit before the test.
	localClearMailpit(t, mailpit)

	// Derive tenant+workspace from the API key's send path. We assume the key
	// already has a workspace bound — the test caller must provide a valid key.
	// We build the ref in the form "tenant:workspace:typeSlug".
	// For management calls we use the manage API; for send we use the data plane.

	typeSlug := fmt.Sprintf("ui-bulk-%d", time.Now().UnixNano())

	// Create a bulk template_type via manage API (requires OIDC/manage; fall
	// back to a superadmin token). Since the dev stack uses Keycloak, we
	// send the API key as Bearer for the manage path — it will likely 401
	// unless the key holder is also a manage-scope user. A simpler approach
	// is to have the caller pass a manage token via a separate env var, or
	// use a superadmin token. For the local dev flow, the caller is expected
	// to provide a key with at least workspace_editor rights obtained from
	// the manage UI.
	//
	// The data plane (POST /api/v1/send) accepts raw API keys. Management
	// endpoints (POST /api/v1/manage/...) require OIDC or superadmin token.
	//
	// For simplicity, this test uses the data-plane API key for sends, and
	// expects the template_type + template to already exist (created manually
	// or by a prior `make dev` setup step). If the type does not exist the
	// test fails with a clear message.
	//
	// To pre-create the type, run from the backend:
	//   curl -s -X POST $SENDA_E2E_LOCAL_BACKEND/api/v1/send \
	//     -H "Authorization: Bearer $SENDA_E2E_LOCAL_API_KEY" \
	//     ...
	//
	// Advanced operators can pass SENDA_E2E_LOCAL_MANAGE_TOKEN to bypass this.
	manageToken := os.Getenv("SENDA_E2E_LOCAL_MANAGE_TOKEN")

	tenantCode := localEnvOr("SENDA_E2E_LOCAL_TENANT", "test-corp")
	workspaceCode := localEnvOr("SENDA_E2E_LOCAL_WORKSPACE", "main")

	if manageToken != "" {
		// Create the template type + template programmatically.
		managePath := fmt.Sprintf("%s/api/v1/manage/tenants/%s/workspaces/%s", backend, tenantCode, workspaceCode)

		// Resolve the workspace's default adapter so the new template_type can send.
		listAdaptersResp := localAPICall(t, http.MethodGet, managePath+"/adapters", manageToken, nil)
		adaptersBody, _ := io.ReadAll(listAdaptersResp.Body)
		listAdaptersResp.Body.Close()
		require.Equal(t, http.StatusOK, listAdaptersResp.StatusCode, "list adapters: %s", adaptersBody)
		var adaptersList struct {
			Items []struct {
				ID        string `json:"id"`
				IsDefault bool   `json:"is_default"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(adaptersBody, &adaptersList))
		var adapterID string
		for _, a := range adaptersList.Items {
			if a.IsDefault {
				adapterID = a.ID
				break
			}
		}
		if adapterID == "" && len(adaptersList.Items) > 0 {
			adapterID = adaptersList.Items[0].ID
		}
		require.NotEmpty(t, adapterID, "workspace must have at least one adapter (set is_default on it)")

		createTTResp := localAPICall(t, http.MethodPost, managePath+"/template-types", manageToken, map[string]any{
			"slug":       typeSlug,
			"name":       "UI Unsubscribe Bulk Test",
			"is_bulk":    true,
			"adapter_id": adapterID,
		})
		createTTResp.Body.Close()
		require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, createTTResp.StatusCode,
			"create template type failed with %d", createTTResp.StatusCode)

		// Get the template type ID.
		getTTResp := localAPICall(t, http.MethodGet, managePath+"/template-types/"+typeSlug, manageToken, nil)
		body, _ := io.ReadAll(getTTResp.Body)
		getTTResp.Body.Close()
		require.Equal(t, http.StatusOK, getTTResp.StatusCode, "get template type: %s", body)
		var ttInfo struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(body, &ttInfo))

		// Create template.
		createTplResp := localAPICall(t, http.MethodPost, managePath+"/templates", manageToken, map[string]any{
			"template_type_id": ttInfo.ID,
		})
		createTplResp.Body.Close()

		// Get template ID.
		listTplResp := localAPICall(t, http.MethodGet, managePath+"/template-types/"+typeSlug+"/templates", manageToken, nil)
		tplBody, _ := io.ReadAll(listTplResp.Body)
		listTplResp.Body.Close()
		var tplList struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(tplBody, &tplList))
		require.NotEmpty(t, tplList.Items, "no templates found for type %s", typeSlug)
		templateID := tplList.Items[0].ID

		// Create a version with the unsubscribe URL variable.
		mjml := `<mjml><mj-body><mj-section><mj-column>` +
			`<mj-text>Hello {{ event.name }}. ` +
			`<a href="{{ system.unsubscribe_url }}">Unsubscribe</a>` +
			`</mj-text></mj-column></mj-section></mj-body></mjml>`
		createVerResp := localAPICall(t, http.MethodPost,
			fmt.Sprintf("%s/templates/%s/versions", managePath, templateID),
			manageToken,
			map[string]any{
				"subject":        "UI E2E Test",
				"preview_text":   "UI E2E",
				"from_email":     "noreply@mail.test.example.com",
				"from_name":      "Test Corp",
				"body_mjml":      mjml,
				"default_locale": "en",
			})
		vBody, _ := io.ReadAll(createVerResp.Body)
		createVerResp.Body.Close()
		require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, createVerResp.StatusCode,
			"create version: %s", vBody)
		var verInfo struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(vBody, &verInfo)

		if verInfo.ID != "" {
			// Set the en locale.
			localeResp := localAPICall(t, http.MethodPut,
				fmt.Sprintf("%s/templates/%s/versions/%s/locales/en", managePath, templateID, verInfo.ID),
				manageToken,
				map[string]any{
					"subject":      "UI E2E Test",
					"preview_text": "UI E2E",
					"from_name":    "Test Corp",
					"body_mjml":    mjml,
				})
			localeResp.Body.Close()

			// Publish.
			pubResp := localAPICall(t, http.MethodPost,
				fmt.Sprintf("%s/templates/%s/versions/%s/publish", managePath, templateID, verInfo.ID),
				manageToken, nil)
			pubResp.Body.Close()
		}
	} else {
		// No manage token — assume the template_type already exists.
		// Use a well-known slug from env or fall back to a default.
		typeSlug = localEnvOr("SENDA_E2E_LOCAL_TYPE_SLUG", typeSlug)
		t.Logf("SENDA_E2E_LOCAL_MANAGE_TOKEN not set; assuming template type %q already exists", typeSlug)
		t.Logf("Set SENDA_E2E_LOCAL_MANAGE_TOKEN to create the type automatically.")
	}

	// Send an email via the data plane.
	recipient := fmt.Sprintf("ui-recipient+%d@e2e.test", time.Now().UnixNano())
	ref := fmt.Sprintf("%s:%s:%s", tenantCode, workspaceCode, typeSlug)

	sendResp := localAPICall(t, http.MethodPost, backend+"/api/v1/send", apiKey, map[string]any{
		"ref":       ref,
		"to":        []string{recipient},
		"variables": map[string]any{"name": "UI E2E User"},
	})
	sendBody, _ := io.ReadAll(sendResp.Body)
	sendResp.Body.Close()
	require.Equal(t, http.StatusAccepted, sendResp.StatusCode,
		"send failed: %s", sendBody)

	// Wait for the token in Mailpit.
	token := localWaitForToken(t, mailpit, recipient, 30*time.Second)
	t.Logf("Extracted unsubscribe token: %s", token)

	// Set up a chromedp context.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
		)...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 90*time.Second)
	defer cancelTimeout()

	// Step 1: Navigate to /u/{token} and click "unsubscribe from this event".
	var titleText string
	unsubURL := frontend + "/u/" + token
	t.Logf("Navigating to unsubscribe page: %s", unsubURL)

	require.NoError(t, chromedp.Run(ctx,
		chromedp.Navigate(unsubURL),
		chromedp.WaitVisible(`[data-testid="unsubscribe-card"]`, chromedp.ByQuery),
		chromedp.Text(
			`[data-testid="unsubscribe-card"]`,
			&titleText,
			chromedp.NodeVisible, chromedp.ByQuery,
		),
		// Click the "this event type" radio (not opt-out-all).
		chromedp.Click(`[data-testid="radio-this-event"]`, chromedp.ByQuery),
		chromedp.Click(`[data-testid="confirm-button"]`, chromedp.ByQuery),
		// Wait for the success confirmation card.
		chromedp.WaitVisible(`[data-testid="success-card"]`, chromedp.ByQuery),
	))
	t.Logf("Unsubscribe card text: %s", titleText)
	require.True(t,
		strings.Contains(strings.ToLower(titleText), "unsubscrib") ||
			strings.Contains(strings.ToLower(titleText), "opt out"),
		"page must mention unsubscribe/opt-out, got: %s", titleText)

	// Step 2: Verify the second email of the same type is suppressed.
	localClearMailpit(t, mailpit)
	sendResp2 := localAPICall(t, http.MethodPost, backend+"/api/v1/send", apiKey, map[string]any{
		"ref":       ref,
		"to":        []string{recipient},
		"variables": map[string]any{"name": "Should be suppressed"},
	})
	sendResp2.Body.Close()
	time.Sleep(3 * time.Second)
	suppressed := localFilterByRecipient(localGetMessages(t, mailpit), recipient)
	require.Empty(t, suppressed, "after opt-out, no email of this type must reach the recipient")

	// Step 3: Navigate to /u/{token}/preferences and re-enable the subscription.
	prefsURL := frontend + "/u/" + token + "/preferences"
	t.Logf("Navigating to preferences page: %s", prefsURL)

	require.NoError(t, chromedp.Run(ctx,
		chromedp.Navigate(prefsURL),
		chromedp.WaitVisible(`[data-testid="preferences-card"]`, chromedp.ByQuery),
		// Find the checkbox for this type and toggle it back on.
		chromedp.WaitVisible(fmt.Sprintf(`[data-testid="pref-cb-%s"]`, typeSlug), chromedp.ByQuery),
		chromedp.Click(fmt.Sprintf(`[data-testid="pref-cb-%s"]`, typeSlug), chromedp.ByQuery),
		chromedp.Click(`[data-testid="save-button"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	))

	// Step 4: Delivery must resume after re-subscribe.
	localClearMailpit(t, mailpit)
	sendResp3 := localAPICall(t, http.MethodPost, backend+"/api/v1/send", apiKey, map[string]any{
		"ref":       ref,
		"to":        []string{recipient},
		"variables": map[string]any{"name": "After resubscribe"},
	})
	sendResp3.Body.Close()
	require.Eventually(t, func() bool {
		return len(localFilterByRecipient(localGetMessages(t, mailpit), recipient)) > 0
	}, 30*time.Second, 250*time.Millisecond,
		"after re-subscribe, email must be delivered to %s", recipient)
	t.Logf("Re-subscription delivery confirmed for %s", recipient)
}
