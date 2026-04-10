//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	externalProfileSlug = "partner-portal"
	externalToken       = "external-e2e-token"
	externalViewerToken = "external-e2e-viewer"
)

func TestExternalIntegration01_APIHappyChaosAndMaliciousFlows(t *testing.T) {
	ctx := t.Context()
	_, err := ensureCoreHarness(ctx)
	require.NoError(t, err)

	ensureExternalSetup(t)

	admin := NewTestClient(t)
	admin.LoginAs(SuperadminEmail)
	ensureExternalIntegrationProfile(t, admin)

	templateTypeID := MustEnsureTemplateType(t, admin, TenantCode, WorkspaceCode, TemplateTypeSlug, TemplateTypeName, TemplateTypeDesc, "")
	templateID := MustEnsureTemplate(t, admin, TenantCode, WorkspaceCode, templateTypeID)
	draftVersionID := createDraftVersion(t, admin, TenantCode, WorkspaceCode, templateID)

	t.Run("success_list_templates_and_versions", func(t *testing.T) {
		resp := externalRequest(t, http.MethodGet,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/template-types/%s/templates", TemplateTypeSlug),
			nil,
			map[string]string{"X-Tenant-Code": TenantCode},
			"")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var body struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		ParseJSONResponse(t, resp, &body)
		require.NotEmpty(t, body.Items)

		resp = externalRequest(t, http.MethodGet,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/versions/%s", templateID, draftVersionID),
			nil,
			map[string]string{"X-Tenant-Code": TenantCode},
			"")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("success_update_preview_test_send_and_publish", func(t *testing.T) {
		updateResp := externalRequest(t, http.MethodPut,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/versions/%s", templateID, draftVersionID),
			map[string]any{
				"subject":        "External Updated Subject",
				"preview_text":   "Updated preview",
				"from_name":      TestFromName,
				"body_mjml":      SampleMJML(),
				"default_locale": "en",
			},
			map[string]string{"X-Tenant-Code": TenantCode},
			"",
		)
		defer updateResp.Body.Close()
		RequireStatus(t, updateResp, http.StatusOK)

		previewResp := externalRequest(t, http.MethodPost,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/preview-mjml", templateID),
			map[string]any{"mjml": SampleMJML()},
			map[string]string{"X-Tenant-Code": TenantCode},
			"",
		)
		defer previewResp.Body.Close()
		RequireStatus(t, previewResp, http.StatusOK)

		var previewBody struct {
			HTML string `json:"html"`
		}
		ParseJSONResponse(t, previewResp, &previewBody)
		require.NotEmpty(t, previewBody.HTML)

		testSendResp := externalRequest(t, http.MethodPost,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/test-send", templateID),
			map[string]any{
				"recipient_email": "user@example.com",
				"variables": map[string]any{
					"first_name":   "Ada",
					"company_name": "Tether",
				},
			},
			map[string]string{"X-Tenant-Code": TenantCode},
			"",
		)
		defer testSendResp.Body.Close()
		RequireStatus(t, testSendResp, http.StatusOK)

		publishResp := externalRequest(t, http.MethodPost,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/versions/%s/publish", templateID, draftVersionID),
			nil,
			map[string]string{"X-Tenant-Code": TenantCode},
			"",
		)
		defer publishResp.Body.Close()
		require.Equal(t, http.StatusNoContent, publishResp.StatusCode, "publish should succeed: %s", ReadResponseBody(t, publishResp))
	})

	t.Run("chaos_fallback_to_system_is_read_only", func(t *testing.T) {
		readResp := externalRequest(t, http.MethodGet,
			externalWorkspacePath("missing")+"/template-types",
			nil,
			map[string]string{"X-Tenant-Code": TenantCode},
			"fallback=system",
		)
		defer readResp.Body.Close()
		RequireStatus(t, readResp, http.StatusOK)

		writeResp := externalRequest(t, http.MethodPut,
			externalWorkspacePath("missing")+fmt.Sprintf("/templates/%s/versions/%s", templateID, createDraftVersion(t, admin, TenantCode, WorkspaceCode, templateID)),
			map[string]any{
				"subject":        "Should Fail",
				"preview_text":   "Should Fail",
				"from_name":      TestFromName,
				"body_mjml":      SampleMJML(),
				"default_locale": "en",
			},
			map[string]string{"X-Tenant-Code": TenantCode},
			"fallback=system",
		)
		defer writeResp.Body.Close()
		require.Equal(t, http.StatusForbidden, writeResp.StatusCode, "fallback mutation must be blocked: %s", ReadResponseBody(t, writeResp))
	})

	t.Run("malicious_missing_header_invalid_token_and_tenant_spoof_are_denied", func(t *testing.T) {
		resp := externalRequest(t, http.MethodGet,
			externalWorkspacePath(WorkspaceCode)+"/template-types",
			nil,
			nil,
			"",
		)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "missing header should fail: %s", ReadResponseBody(t, resp))

		resp = externalRequest(t, http.MethodGet,
			externalWorkspacePath(WorkspaceCode)+"/template-types",
			nil,
			map[string]string{"X-Tenant-Code": TenantCode},
			"token=wrong-token",
		)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode, "invalid token should fail: %s", ReadResponseBody(t, resp))

		resp = externalRequest(t, http.MethodGet,
			externalWorkspacePath(WorkspaceCode)+"/template-types",
			nil,
			map[string]string{"X-Tenant-Code": "other-tenant"},
			"",
		)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode, "tenant spoof should fail: %s", ReadResponseBody(t, resp))
	})

	t.Run("viewer_permissions_allow_read_and_block_mutations", func(t *testing.T) {
		readResp := externalRequest(t, http.MethodGet,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/versions/%s", templateID, draftVersionID),
			nil,
			map[string]string{
				"X-Tenant-Code":           TenantCode,
				"X-Senda-External-Token": externalViewerToken,
			},
			"",
		)
		defer readResp.Body.Close()
		RequireStatus(t, readResp, http.StatusOK)

		updateResp := externalRequest(t, http.MethodPut,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/versions/%s", templateID, draftVersionID),
			map[string]any{
				"subject":        "Viewer Should Fail",
				"preview_text":   "Viewer Should Fail",
				"from_name":      TestFromName,
				"body_mjml":      SampleMJML(),
				"default_locale": "en",
			},
			map[string]string{
				"X-Tenant-Code":           TenantCode,
				"X-Senda-External-Token": externalViewerToken,
			},
			"",
		)
		defer updateResp.Body.Close()
		require.Equal(t, http.StatusForbidden, updateResp.StatusCode, "viewer update should fail: %s", ReadResponseBody(t, updateResp))

		publishResp := externalRequest(t, http.MethodPost,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/versions/%s/publish", templateID, draftVersionID),
			nil,
			map[string]string{
				"X-Tenant-Code":           TenantCode,
				"X-Senda-External-Token": externalViewerToken,
			},
			"",
		)
		defer publishResp.Body.Close()
		require.Equal(t, http.StatusForbidden, publishResp.StatusCode, "viewer publish should fail: %s", ReadResponseBody(t, publishResp))

		testSendResp := externalRequest(t, http.MethodPost,
			externalWorkspacePath(WorkspaceCode)+fmt.Sprintf("/templates/%s/test-send", templateID),
			map[string]any{
				"recipient_email": "user@example.com",
				"variables": map[string]any{
					"first_name": "Ada",
				},
			},
			map[string]string{
				"X-Tenant-Code":           TenantCode,
				"X-Senda-External-Token": externalViewerToken,
			},
			"",
		)
		defer testSendResp.Body.Close()
		require.Equal(t, http.StatusForbidden, testSendResp.StatusCode, "viewer test-send should fail: %s", ReadResponseBody(t, testSendResp))
	})
}

func ensureExternalIntegrationProfile(t *testing.T, client *TestClient) {
	t.Helper()

	resp := client.Put("/api/v1/manage/config", map[string]any{
		"external_integrations": map[string]any{
			"profiles": []map[string]any{
				{
					"slug":             externalProfileSlug,
					"name":             "Partner Portal",
					"description":      "E2E external integration profile",
					"enabled":          true,
					"auth_method_name": "e2e-signed-token",
					"resolver_name":    "e2e-workspace-resolver",
					"allowed_origins":  []string{"http://localhost:3000"},
					"allowed_headers":  []string{"x-tenant-code"},
					"required_headers": []string{"x-tenant-code"},
					"capabilities": map[string]bool{
						"list_templates":   true,
						"view_versions":    true,
						"edit_versions":    true,
						"publish_versions": true,
						"test_send":        true,
						"builder_access":   true,
						"metadata_access":  true,
						"locale_access":    true,
					},
				},
			},
		},
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

func ensureExternalSetup(t *testing.T) {
	t.Helper()
	WaitForServer(t, 30*time.Second)
	WaitForMailpit(t, 30*time.Second)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	onboardingResp := client.Post("/api/v1/onboarding/setup", map[string]string{
		"tenant_code": TenantCode,
		"tenant_name": TenantName,
	})
	require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, onboardingResp.StatusCode,
		"expected 201 or 409 onboarding setup, got %d: %s", onboardingResp.StatusCode, ReadResponseBody(t, onboardingResp))
	onboardingResp.Body.Close()

	workspaceResp := client.Post(fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", TenantCode), map[string]string{
		"code": WorkspaceCode,
		"name": WorkspaceName,
	})
	require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, workspaceResp.StatusCode,
		"expected 201 or 409 workspace setup, got %d: %s", workspaceResp.StatusCode, ReadResponseBody(t, workspaceResp))
	workspaceResp.Body.Close()

	adapterID := mustEnsureWorkspaceAdapter(t, client, TenantCode, WorkspaceCode, 100)
	EnsureDefaultAdapterIdentity(t, adapterID, TestFromEmail)

	templateTypeID := MustEnsureTemplateType(t, client, TenantCode, WorkspaceCode, TemplateTypeSlug, TemplateTypeName, TemplateTypeDesc, adapterID)
	templateID := MustEnsureTemplate(t, client, TenantCode, WorkspaceCode, templateTypeID)
	_ = MustEnsureVersionPublished(t, client, TenantCode, WorkspaceCode, templateID)
}

func createDraftVersion(t *testing.T, client *TestClient, tenantCode, workspaceCode, templateID string) string {
	t.Helper()

	resp := client.Post(fmt.Sprintf("%s/templates/%s/versions", mustWorkspacePath(tenantCode, workspaceCode), templateID), CreateVersionRequest{
		Subject:       "External Draft Subject",
		PreviewText:   "External draft preview",
		FromName:      TestFromName,
		BodyMJML:      SampleMJML(),
		DefaultLocale: "en",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var body struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &body)
	require.NotEmpty(t, body.ID)
	return body.ID
}

func externalWorkspacePath(workspaceCode string) string {
	return fmt.Sprintf("/api/v1/external/%s/tenants/%s/workspaces/%s", externalProfileSlug, TenantCode, workspaceCode)
}

func externalRequest(t *testing.T, method, path string, body any, headers map[string]string, extraQuery string) *http.Response {
	t.Helper()

	client := NewTestClient(t)
	url := client.baseURL + path
	separator := "?"
	if bytes.Contains([]byte(url), []byte("?")) {
		separator = "&"
	}
	query := ""
	if extraQuery != "" {
		query += extraQuery
	}
	if query != "" {
		url += separator + query
	}

	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}

	req, err := http.NewRequest(method, url, &payload)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	_, hasCanonicalTokenHeader := headers["X-Senda-External-Token"]
	_, hasLowerTokenHeader := headers["x-senda-external-token"]
	if !hasCanonicalTokenHeader && !hasLowerTokenHeader && !bytes.Contains([]byte(extraQuery), []byte("token=")) {
		req.Header.Set("X-Senda-External-Token", externalToken)
	}

	resp, err := client.httpClient.Do(req)
	require.NoError(t, err)
	return resp
}
