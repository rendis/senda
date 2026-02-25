//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createAPIKeyInWorkspace(t *testing.T, client *TestClient, workspaceCode, suffix string) string {
	t.Helper()
	path := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s/api-keys", TenantCode, workspaceCode)
	req := APIKeyRequest{
		Name: APIKeyNamePrefix + fmt.Sprintf("%s-%s-%d", workspaceCode, suffix, time.Now().UnixNano()),
	}

	resp := client.Post(path, req)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var body struct {
		Key   string `json:"key"`
		Token string `json:"token"`
	}
	ParseJSONResponse(t, resp, &body)

	key := body.Key
	if key == "" {
		key = body.Token
	}
	require.NotEmpty(t, key, "api key value is required")
	return key
}

func TestCore01_Send_CrossWorkspaceForbidden(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	// Ensure a second workspace exists.
	resp := client.Post(fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", TenantCode), map[string]string{
		"code": "otherws",
		"name": "Other Workspace",
	})
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict,
		"expected 201 or 409, got %d", resp.StatusCode)

	otherKey := createAPIKeyInWorkspace(t, client, "otherws", "cross-scope")

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(otherKey)

	sendResp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  []string{"cross-scope@test.example.com"},
		Variables: map[string]interface{}{
			"first_name":   "Cross",
			"company_name": "Scope",
		},
	})
	defer sendResp.Body.Close()

	RequireStatus(t, sendResp, http.StatusForbidden)
	AssertError(t, sendResp, "FORBIDDEN")
}

func TestCore02_Send_SystemWorkspaceBlocked(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	systemKey := createAPIKeyInWorkspace(t, client, SystemWorkspaceCode, "system-block")
	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(systemKey)

	systemRef := fmt.Sprintf("%s:%s:%s", TenantCode, SystemWorkspaceCode, TemplateTypeSlug)
	sendResp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: systemRef,
		To:  []string{"system-workspace@test.example.com"},
		Variables: map[string]interface{}{
			"first_name":   "System",
			"company_name": "Workspace",
		},
	})
	defer sendResp.Body.Close()

	RequireStatus(t, sendResp, http.StatusUnprocessableEntity)
	AssertError(t, sendResp, "SYSTEM_WORKSPACE_BLOCKED")
}

func TestCore03_Send_RecipientLimit(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	apiKey := createAPIKey(t, client, "recipient-limit")

	recipients := make([]string, 51)
	for i := range recipients {
		recipients[i] = fmt.Sprintf("limit-%d@test.example.com", i)
	}

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKey)
	sendResp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  recipients,
		Variables: map[string]interface{}{
			"first_name":   "Limit",
			"company_name": "Test",
		},
	})
	defer sendResp.Body.Close()

	RequireStatus(t, sendResp, http.StatusUnprocessableEntity)
	AssertError(t, sendResp, "VALIDATION_ERROR")
}

func TestCore04_Send_TemplateDisabledConflict(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	ttResp := client.Get(fmt.Sprintf("%s/template-types/%s", wsPath(), TemplateTypeSlug))
	defer ttResp.Body.Close()
	RequireStatus(t, ttResp, http.StatusOK)

	var tt struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, ttResp, &tt)
	require.NotEmpty(t, tt.ID)

	templateID := GetTemplateIDByTypeID(t, tt.ID)
	require.NotEmpty(t, templateID)

	disableResp := client.Post(fmt.Sprintf("%s/templates/%s/disable", wsPath(), templateID), nil)
	defer disableResp.Body.Close()
	RequireStatus(t, disableResp, http.StatusNoContent)

	t.Cleanup(func() {
		enableResp := client.Post(fmt.Sprintf("%s/templates/%s/enable", wsPath(), templateID), nil)
		if enableResp != nil {
			enableResp.Body.Close()
		}
	})

	apiKey := createAPIKey(t, client, "disabled-template")
	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKey)

	sendResp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  []string{"disabled-template@test.example.com"},
		Variables: map[string]interface{}{
			"first_name":   "Disabled",
			"company_name": "Template",
		},
	})
	defer sendResp.Body.Close()

	RequireStatus(t, sendResp, http.StatusConflict)
	AssertError(t, sendResp, "TEMPLATE_DISABLED")
}

func TestCore05_DataPlane_ScopeIsolation(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	resp := client.Post(fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", TenantCode), map[string]string{
		"code": "scopews",
		"name": "Scope Workspace",
	})
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict,
		"expected 201 or 409, got %d", resp.StatusCode)

	mainKey := createAPIKey(t, client, "scope-main")
	scopeKey := createAPIKeyInWorkspace(t, client, "scopews", "scope-alt")

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(mainKey)
	sendResp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  []string{"scope-main@test.example.com"},
		Variables: map[string]interface{}{
			"first_name":   "Scope",
			"company_name": "Main",
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

	otherClient := NewTestClient(t)
	otherClient.SetAPIKey(scopeKey)

	listResp := otherClient.Get("/api/v1/emails")
	defer listResp.Body.Close()
	RequireStatus(t, listResp, http.StatusOK)

	var listBody struct {
		Items []struct {
			TrackingID string `json:"tracking_id"`
		} `json:"items"`
	}
	ParseJSONResponse(t, listResp, &listBody)
	for _, item := range listBody.Items {
		require.NotEqual(t, trackingID, item.TrackingID, "cross-workspace email leaked in data-plane list")
	}

	detailResp := otherClient.Get("/api/v1/emails/" + trackingID)
	defer detailResp.Body.Close()
	RequireStatus(t, detailResp, http.StatusNotFound)
}
