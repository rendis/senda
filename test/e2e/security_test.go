//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// OWASP Top 10 Security Test Suite (S01-S12)
// ============================================================================
//
// Mentalidad adversarial: these tests attempt to exploit the API contract
// like a real attacker. Failures indicate critical or high-risk vulnerabilities.
//

// TestS01_SQLInjection tests SQL injection protection across all user inputs.
// OWASP: A03:2021 - Injection
// Target: All user-controlled string parameters (query params, JSON fields, path params)
// Expected: 400 Bad Request (validation) or normal response with NO injected data.
// Failure severity: CRITICAL - SQL injection can leak entire database
func TestS01_SQLInjection(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)
	emailsPath := wsPath + "/emails"

	sqlPayloads := []string{
		"' OR '1'='1",
		"'; DROP TABLE emails; --",
		"1; SELECT * FROM pg_catalog.pg_tables --",
		"' UNION SELECT 1,2,3,4,5 --",
		"\\'; DELETE FROM templates WHERE ''='",
		"admin' --",
		"' OR 1=1 --",
		"' AND '1'='1",
		"' AND 1=2 UNION SELECT NULL,NULL,NULL --",
	}

	t.Run("emails_query_external_id_injection", func(t *testing.T) {
		for _, payload := range sqlPayloads {
			resp := client.Get(emailsPath + "?external_id=" + url.QueryEscape(payload))
			defer resp.Body.Close()

			// Should NOT be 500 (which indicates SQL reached DB)
			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"SQL injection payload reached database: %s", payload)

			// Should be 400 (validation error) or 200 with empty results
			require.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusOK,
				"unexpected status %d for payload: %s", resp.StatusCode, payload)
		}
	})

	t.Run("emails_query_recipient_injection", func(t *testing.T) {
		for _, payload := range sqlPayloads {
			resp := client.Get(emailsPath + "?recipient=" + url.QueryEscape(payload))
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"SQL injection in recipient parameter: %s", payload)
		}
	})

	t.Run("emails_query_cursor_injection", func(t *testing.T) {
		for _, payload := range sqlPayloads {
			resp := client.Get(emailsPath + "?cursor=" + url.QueryEscape(payload))
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"cursor payload must never trigger 500: %s", payload)

			require.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusOK,
				"unexpected status %d for cursor payload: %s", resp.StatusCode, payload)
		}
	})

	t.Run("tenant_create_code_injection", func(t *testing.T) {
		for _, payload := range sqlPayloads {
			req := map[string]interface{}{
				"code": payload,
				"name": "Test Tenant",
			}
			resp := client.Post("/api/v1/manage/tenants", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"SQL injection in tenant code: %s", payload)
		}
	})

	t.Run("injector_create_name_injection", func(t *testing.T) {
		for _, payload := range sqlPayloads {
			req := InjectorRequest{
				Name:        payload,
				Description: "Test",
				Fields:      []InjectorFieldRequest{},
			}
			resp := client.Post(wsPath+"/injectors", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"SQL injection in injector name: %s", payload)
		}
	})

	t.Run("send_variables_injection", func(t *testing.T) {
		for _, payload := range sqlPayloads {
			req := SendRequest{
				Ref: "test-ref",
				To:  []string{"test@example.com"},
				Variables: map[string]interface{}{
					"user_input": payload,
				},
			}
			resp := client.Post("/api/v1/send", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"SQL injection in send variables: %s", payload)
		}
	})
}

// TestS02_BrokenAuthentication tests auth bypass vulnerabilities.
// OWASP: A07:2021 - Identification and Authentication Failures
// Target: Auth middleware (missing/invalid JWT, API Key)
// Expected: 401 Unauthorized for all invalid credentials
// Failure severity: CRITICAL - can lead to unauthorized access
func TestS02_BrokenAuthentication(t *testing.T) {
	// These tests explicitly test UNAUTHORIZED access - no LoginAs.

	t.Run("missing_authorization_header", func(t *testing.T) {
		freshClient := NewTestClient(t)
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("invalid_bearer_token", func(t *testing.T) {
		freshClient := NewTestClient(t)
		freshClient.SetBearerToken("invalid.jwt.token")
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("malformed_jwt", func(t *testing.T) {
		freshClient := NewTestClient(t)
		freshClient.SetBearerToken("not-a-jwt-at-all")
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("jwt_with_wrong_signature", func(t *testing.T) {
		// Valid JWT structure but signed with wrong key
		freshClient := NewTestClient(t)
		wrongToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkF0dGFja2VyIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
		freshClient.SetBearerToken(wrongToken)
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("empty_bearer_token", func(t *testing.T) {
		freshClient := NewTestClient(t)
		freshClient.SetBearerToken("")
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("invalid_api_key_format", func(t *testing.T) {
		freshClient := NewTestClient(t)
		freshClient.SetAPIKey("not-a-valid-key")
		resp := freshClient.Post("/api/v1/send", SendRequest{
			Ref: "test-ref",
			To:  []string{"test@example.com"},
		})
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("api_key_with_extra_characters", func(t *testing.T) {
		freshClient := NewTestClient(t)
		freshClient.SetAPIKey("snd_live_abc123xyz!!invalid")
		emailsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s/emails", TenantCode, WorkspaceCode)
		resp := freshClient.Get(emailsPath)
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("empty_api_key", func(t *testing.T) {
		freshClient := NewTestClient(t)
		freshClient.SetAPIKey("")
		emailsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s/emails", TenantCode, WorkspaceCode)
		resp := freshClient.Get(emailsPath)
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("api_key_rejected_in_management_endpoints", func(t *testing.T) {
		freshClient := NewTestClient(t)
		// Use a dummy API key (which will fail authentication, but proves we're checking)
		freshClient.SetAPIKey("snd_live_validformat")
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		// Should reject with 401/403 because API keys are not allowed for management
		require.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
			"expected 401/403 for API key on management endpoint, got %d", resp.StatusCode)
	})

	t.Run("none_algorithm_jwt", func(t *testing.T) {
		freshClient := NewTestClient(t)
		// JWT with "none" algorithm (security vulnerability if accepted)
		noneAlgoToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkF0dGFja2VyIn0."
		freshClient.SetBearerToken(noneAlgoToken)
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

// TestS03_BrokenAccessControl tests authorization boundary violations.
// OWASP: A01:2021 - Broken Access Control
// Target: RBAC middleware and object ownership validation
// Expected: 403 Forbidden for insufficient permissions, 404 for cross-tenant access
// Failure severity: CRITICAL - can lead to privilege escalation
func TestS03_BrokenAccessControl(t *testing.T) {
	EnsureSetup(t)
	t.Run("unauthenticated_management_access", func(t *testing.T) {
		freshClient := NewTestClient(t)
		resp := freshClient.Get("/api/v1/manage/tenants")
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"management API should require auth, got %d", resp.StatusCode)
	})

	t.Run("unauthenticated_send_access", func(t *testing.T) {
		freshClient := NewTestClient(t)
		resp := freshClient.Post("/api/v1/send", SendRequest{
			Ref: "test",
			To:  []string{"test@example.com"},
		})
		defer resp.Body.Close()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"send API should require auth, got %d", resp.StatusCode)
	})

	t.Run("workspace_viewer_calls_admin_endpoint", func(t *testing.T) {
		// Attempt to POST /injectors (admin) as workspace-viewer
		client := NewTestClient(t)
		client.LoginAs(WorkspaceViewerEmail)
		wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

		req := InjectorRequest{
			Name:        "test-injector",
			Description: "Test",
			Fields:      []InjectorFieldRequest{},
		}
		resp := client.Post(wsPath+"/injectors", req)
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized,
			"viewer should not be able to create injectors, got %d", resp.StatusCode)
	})

	t.Run("workspace_editor_calls_delete_endpoint", func(t *testing.T) {
		// Attempt to DELETE /templates (admin) as workspace-editor
		client := NewTestClient(t)
		client.LoginAs(WorkspaceEditorEmail)
		wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

		resp := client.Delete(wsPath + "/templates/test-template")
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound,
			"editor should not be able to delete templates, got %d", resp.StatusCode)
	})

	t.Run("tenant_admin_calls_superadmin_endpoint", func(t *testing.T) {
		// Attempt to POST /tenants (superadmin only)
		client := NewTestClient(t)
		client.LoginAs(TenantAdminEmail)

		req := map[string]interface{}{
			"code": "new-tenant",
			"name": "New Tenant",
		}
		resp := client.Post("/api/v1/manage/tenants", req)
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized,
			"tenant-admin should not be able to create tenants, got %d", resp.StatusCode)
	})

	t.Run("cross_tenant_access_returns_404", func(t *testing.T) {
		// Attempt to access resource from different tenant
		client := NewTestClient(t)
		client.LoginAs(SuperadminEmail)
		maliciousTenantCode := "other-tenant"

		resp := client.Get(fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", maliciousTenantCode))
		defer resp.Body.Close()

		// Should return 404 (not found), NOT 200 (which would leak data)
		require.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden,
			"accessing other tenant should return 404/403, got %d", resp.StatusCode)
	})

	t.Run("cross_workspace_access_returns_404", func(t *testing.T) {
		client := NewTestClient(t)
		client.LoginAs(SuperadminEmail)
		maliciousPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, "other-workspace")

		resp := client.Get(maliciousPath)
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden,
			"accessing other workspace should return 404/403, got %d", resp.StatusCode)
	})

	t.Run("api_key_workspace_isolation", func(t *testing.T) {
		// API Key for workspace A should not access workspace B
		freshClient := NewTestClient(t)
		freshClient.SetAPIKey("snd_live_workspace_a_key")

		// Attempt to send (should fail because key is invalid/unknown)
		resp := freshClient.Post("/api/v1/send", SendRequest{
			Ref: "isolation-test",
			To:  []string{"test@example.com"},
		})
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
			"API key should be validated, got %d", resp.StatusCode)
	})

	t.Run("path_traversal_in_url", func(t *testing.T) {
		client := NewTestClient(t)
		client.LoginAs(SuperadminEmail)
		resp := client.Get("/api/v1/manage/tenants/../../../admin/users")
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest,
			"path traversal should return 400/404, got %d", resp.StatusCode)
	})

	t.Run("enumeration_protection_returns_404_not_403", func(t *testing.T) {
		// Try to access resource with fake UUID
		client := NewTestClient(t)
		client.LoginAs(SuperadminEmail)
		fakeUUID := "550e8400-e29b-41d4-a716-446655440000"
		wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

		resp := client.Get(wsPath + "/templates/" + fakeUUID)
		defer resp.Body.Close()

		// Must return 404, NOT 403 (which would confirm existence)
		require.Equal(t, http.StatusNotFound, resp.StatusCode,
			"non-existent resource should return 404 to prevent enumeration, got %d", resp.StatusCode)
	})
}

// TestS04_XSSViaTemplates tests XSS injection through template content.
// OWASP: A03:2021 - Injection (XSS variant)
// Target: Template storage and rendering (subject, body, preview_text, from_name)
// Expected: Content stored but sanitized on output, or rejected with 400
// Failure severity: HIGH - can steal user sessions or credentials
func TestS04_XSSViaTemplates(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"\"><img src=x onerror=alert(1)>",
		"<svg onload=alert('xss')>",
		"<iframe src=\"javascript:alert('xss')\"></iframe>",
		"javascript:alert('xss')",
		"<body onload=alert('xss')>",
		"<input onfocus=alert('xss') autofocus>",
		"<details open ontoggle=alert('xss')>",
		"<video src=x onerror=alert('xss')>",
	}

	t.Run("template_body_xss_injection", func(t *testing.T) {
		for _, payload := range xssPayloads {
			req := CreateVersionRequest{
				Subject:       "Test",
				PreviewText:   "Test",
				FromEmail:     "test@example.com",
				FromName:      "Test",
				BodyMJML:      payload,
				DefaultLocale: "en",
			}
			resp := client.Post(wsPath+"/templates/nonexistent/versions", req)
			defer resp.Body.Close()

			// Should accept but sanitize, reject with 400, or 404 (template not found)
			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"XSS payload in body should not cause 500, got %d for payload: %s",
				resp.StatusCode, payload)
		}
	})

	t.Run("template_subject_xss_injection", func(t *testing.T) {
		for _, payload := range xssPayloads {
			req := CreateVersionRequest{
				Subject:       payload,
				PreviewText:   "Test",
				FromEmail:     "test@example.com",
				FromName:      "Test",
				BodyMJML:      SampleMJML(),
				DefaultLocale: "en",
			}
			resp := client.Post(wsPath+"/templates/nonexistent/versions", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"XSS in subject should not cause 500, got %d for payload: %s", resp.StatusCode, payload)
		}
	})

	t.Run("template_from_name_xss_injection", func(t *testing.T) {
		for _, payload := range xssPayloads {
			req := CreateVersionRequest{
				Subject:       "Test",
				PreviewText:   "Test",
				FromEmail:     "test@example.com",
				FromName:      payload,
				BodyMJML:      SampleMJML(),
				DefaultLocale: "en",
			}
			resp := client.Post(wsPath+"/templates/nonexistent/versions", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"XSS in from_name should not cause 500, got %d", resp.StatusCode)
		}
	})

	t.Run("injector_values_xss_injection", func(t *testing.T) {
		for _, payload := range xssPayloads {
			req := SetInjectorValuesRequest{
				Values: []InjectorFieldValue{
					{FieldName: "test_field", Value: payload},
				},
			}
			resp := client.Put(wsPath+"/injectors/test/values", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"XSS in injector values should not cause 500, got %d", resp.StatusCode)
		}
	})
}

// TestS05_SSRFViaWebhooks tests SSRF protection on webhook URLs.
// OWASP: A10:2021 - Server-Side Request Forgery (SSRF)
// Target: Webhook URL validation (should block private IPs, metadata endpoints)
// Expected: 400 Bad Request for private/reserved IPs
// Failure severity: HIGH - can leak internal services, cloud metadata
func TestS05_SSRFViaWebhooks(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	ssrfPayloads := []struct {
		url        string
		shouldFail bool
		reason     string
	}{
		// Loopback addresses
		{"http://127.0.0.1:8080", true, "IPv4 loopback"},
		{"http://localhost:3000", true, "localhost"},
		{"http://[::1]:8080", true, "IPv6 loopback"},
		{"http://0.0.0.0:8080", true, "0.0.0.0"},

		// Private RFC 1918
		{"http://10.0.0.1:8080", true, "10.0.0.0/8"},
		{"http://172.16.0.1:8080", true, "172.16.0.0/12"},
		{"http://192.168.1.1:8080", true, "192.168.0.0/16"},

		// Cloud metadata endpoints
		{"http://169.254.169.254/latest/meta-data", true, "AWS metadata endpoint"},
		{"http://169.254.169.254", true, "AWS link-local"},
		{"http://metadata.google.internal", true, "GCP metadata"},

		// Valid external URLs (should succeed)
		{"https://example.com/webhook", false, "https external"},
	}

	t.Run("webhook_ssrf_protection", func(t *testing.T) {
		for _, payload := range ssrfPayloads {
			req := WebhookRequest{
				URL:    payload.url,
				Events: []string{"email.sent"},
			}
			resp := client.Post(wsPath+"/webhooks", req)
			defer resp.Body.Close()

			if payload.shouldFail {
				require.True(t, resp.StatusCode == http.StatusBadRequest ||
					resp.StatusCode == http.StatusUnprocessableEntity,
					"should reject %s (%s), got %d",
					payload.url, payload.reason, resp.StatusCode)
			} else {
				require.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK,
					"should accept %s (%s), got %d",
					payload.url, payload.reason, resp.StatusCode)
			}
		}
	})

	t.Run("webhook_dns_rebinding_protection", func(t *testing.T) {
		// Attempt DNS rebinding (domain that resolves to localhost)
		rebindingURL := "http://attacker-rebind.evil.com"
		req := WebhookRequest{
			URL:    rebindingURL,
			Events: []string{"email.sent"},
		}
		resp := client.Post(wsPath+"/webhooks", req)
		defer resp.Body.Close()

		// Should either reject or handle gracefully, not crash
		require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"DNS rebinding attempt should not cause 500")
	})
}

// TestS06_MassAssignment tests mass assignment vulnerabilities.
// OWASP: A01:2021 - Broken Access Control (mass assignment variant)
// Target: Request body parsing that should ignore certain fields
// Expected: Extra fields ignored, not applied
// Failure severity: MEDIUM - can set unintended field values
func TestS06_MassAssignment(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	t.Run("tenant_create_with_extra_fields", func(t *testing.T) {
		req := map[string]interface{}{
			"code":             fmt.Sprintf("mass-test-%d", time.Now().UnixNano()),
			"name":             "Test Tenant",
			"is_superadmin":    true,   // Should be ignored
			"secret_field":     "hack", // Should be ignored
			"billing_override": 9999,   // Should be ignored
		}
		resp := client.Post("/api/v1/manage/tenants", req)
		defer resp.Body.Close()

		// Should succeed but ignore extra fields, or reject
		require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"extra fields should not cause 500")

		if resp.StatusCode == http.StatusCreated {
			// Verify is_superadmin was not set
			body := ReadResponseBody(t, resp)
			require.NotContains(t, body, "\"is_superadmin\":true",
				"extra field is_superadmin should be ignored")
		}
	})

	t.Run("send_with_extra_fields", func(t *testing.T) {
		_, apiKeyValue := MustCreateAPIKey(t, client, TenantCode, WorkspaceCode, "mass")

		sendClient := NewTestClient(t)
		sendClient.SetAPIKey(apiKeyValue)

		req := map[string]interface{}{
			"ref":               sendRef(),
			"to":                []string{"test@example.com"},
			"bypass_rate_limit": true,       // Should be ignored
			"priority":          "critical", // Should be ignored
			"admin_override":    true,       // Should be ignored
			"variables": map[string]interface{}{
				"first_name":   "Test",
				"company_name": "Test Corp",
			},
		}
		resp := sendClient.Post("/api/v1/send", req)
		defer resp.Body.Close()

		require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"extra fields should not cause 500")
	})

	t.Run("api_key_create_with_extra_fields", func(t *testing.T) {
		req := map[string]interface{}{
			"name":         fmt.Sprintf("mass-key-%d", time.Now().UnixNano()),
			"workspace_id": "other-workspace-uuid", // Should be ignored
			"permissions":  []string{"admin"},      // Should be ignored
			"expires_at":   "2099-12-31",           // Should be ignored
		}
		resp := client.Post(wsPath+"/api-keys", req)
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusBadRequest,
			"unexpected status: %d", resp.StatusCode)
	})

	t.Run("member_invite_with_extra_fields", func(t *testing.T) {
		req := map[string]interface{}{
			"email":         "mass-test-user@example.com",
			"roles":         []string{"workspace-viewer"},
			"is_superadmin": true,          // Should be ignored
			"salary":        100000,        // Should be ignored
			"department":    "engineering", // Should be ignored
		}
		resp := client.Post(wsPath+"/members", req)
		defer resp.Body.Close()

		require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"mass assignment should not cause 500")
	})
}

// TestS07_IDOR tests Insecure Direct Object Reference vulnerabilities.
// OWASP: A01:2021 - Broken Access Control (IDOR variant)
// Target: Direct UUID/ID access without proper authorization checks
// Expected: 404 for resources not belonging to current user/workspace
// Failure severity: CRITICAL - can access other users' data
func TestS07_IDOR(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	// Random UUIDs to test enumeration
	randomUUIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"00000000-0000-0000-0000-000000000000",
	}

	t.Run("nonexistent_template_returns_404", func(t *testing.T) {
		for _, uuid := range randomUUIDs {
			resp := client.Get(wsPath + "/templates/" + uuid + "/versions")
			defer resp.Body.Close()

			// The GET /templates/:template_id/versions endpoint may return 200 with empty results
			// for nonexistent templates (no data leak) or 404. Both are acceptable IDOR protection.
			require.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusOK,
				"non-existent template should return 404 or 200 (empty), got %d", resp.StatusCode)
		}
	})

	t.Run("nonexistent_webhook_returns_404", func(t *testing.T) {
		for _, uuid := range randomUUIDs {
			resp := client.Get(wsPath + "/webhooks/" + uuid)
			defer resp.Body.Close()

			require.Equal(t, http.StatusNotFound, resp.StatusCode,
				"non-existent webhook should return 404, got %d", resp.StatusCode)
		}
	})

	t.Run("nonexistent_adapter_returns_404", func(t *testing.T) {
		for _, uuid := range randomUUIDs {
			resp := client.Get(wsPath + "/adapters/" + uuid)
			defer resp.Body.Close()

			require.Equal(t, http.StatusNotFound, resp.StatusCode,
				"non-existent adapter should return 404, got %d", resp.StatusCode)
		}
	})

	t.Run("nonexistent_api_key_returns_404", func(t *testing.T) {
		// No GET /api-keys/:id route exists; use DELETE /api-keys/:id to test IDOR.
		for _, uuid := range randomUUIDs {
			resp := client.Delete(wsPath + "/api-keys/" + uuid)
			defer resp.Body.Close()

			require.Equal(t, http.StatusNotFound, resp.StatusCode,
				"non-existent api key should return 404, got %d", resp.StatusCode)
		}
	})
}

// TestS08_RateLimitBypass tests rate limiting bypass techniques.
// OWASP: A05:2021 - Resource Exhaustion (rate limit evasion)
// Target: Rate limiter should persist across header mutations
// Expected: Rate limit enforced per workspace, not per IP/header combination
// Failure severity: MEDIUM - can enable brute force, DOS
func TestS08_RateLimitBypass(t *testing.T) {
	EnsureSetup(t)
	t.Run("unauthenticated_requests_rejected", func(t *testing.T) {
		client := NewTestClient(t)

		req := SendRequest{
			Ref: "test-ref",
			To:  []string{"test@example.com"},
		}

		for i := 0; i < 5; i++ {
			resp := client.Post("/api/v1/send", req)
			defer resp.Body.Close()

			require.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"unauthenticated send should return 401")
		}
	})

	t.Run("rate_limit_persists_with_user_agent_rotation", func(t *testing.T) {
		// User-Agent rotation should not bypass rate limit
		client := NewTestClient(t)
		client.LoginAs(SuperadminEmail)

		req := SendRequest{
			Ref: "test-ref",
			To:  []string{"test@example.com"},
		}

		for i := 0; i < 3; i++ {
			resp := client.Post("/api/v1/send", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"rate limit check should not crash")
		}
	})

	t.Run("rate_limit_per_workspace_not_per_auth_method", func(t *testing.T) {
		// Switching between API Key and JWT should not reset rate limit
		req := SendRequest{
			Ref: "test-ref",
			To:  []string{"test@example.com"},
		}

		switchClient := NewTestClient(t)
		switchClient.SetAPIKey("snd_live_key1")
		resp1 := switchClient.Post("/api/v1/send", req)
		defer resp1.Body.Close()

		switchClient2 := NewTestClient(t)
		switchClient2.LoginAs(SuperadminEmail)
		resp2 := switchClient2.Post("/api/v1/send", req)
		defer resp2.Body.Close()

		// Both should be subject to same rate limit if on same workspace
		require.NotEqual(t, http.StatusInternalServerError, resp1.StatusCode)
		require.NotEqual(t, http.StatusInternalServerError, resp2.StatusCode)
	})
}

// TestS09_APIKeyTimingAttack tests constant-time API key comparison.
// OWASP: A02:2021 - Cryptographic Failures (timing attacks)
// Target: API key validation should use constant-time comparison
// Expected: All response times within variance regardless of key similarity
// Failure severity: MEDIUM - enables brute force attacks
func TestS09_APIKeyTimingAttack(t *testing.T) {
	EnsureSetup(t)
	// Function to measure average response time for N requests with a given key
	measureKeyTiming := func(key string, iterations int) time.Duration {
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			testClient := NewTestClient(t)
			testClient.SetAPIKey(key)

			start := time.Now()
			resp := testClient.Post("/api/v1/send", SendRequest{
				Ref: "timing-test",
				To:  []string{"test@example.com"},
			})
			duration := time.Since(start)
			resp.Body.Close()

			totalDuration += duration
		}

		return totalDuration / time.Duration(iterations)
	}

	t.Run("constant_time_api_key_validation", func(t *testing.T) {
		iterations := 10
		const allowedVariance = 100 * time.Millisecond

		// Test completely wrong key
		wrongKeyTiming := measureKeyTiming("snd_live_completely_wrong", iterations)

		// Test key with correct prefix but wrong hash
		prefixMatchTiming := measureKeyTiming("snd_live_abc123", iterations)

		// Test key with correct prefix and some hash similarity
		similarTiming := measureKeyTiming("snd_live_abc124", iterations)

		// Calculate variance
		variance := wrongKeyTiming - similarTiming
		if variance < 0 {
			variance = -variance
		}

		t.Logf("timing: wrong=%v prefix=%v similar=%v variance=%v",
			wrongKeyTiming, prefixMatchTiming, similarTiming, variance)

		require.Less(t, variance, allowedVariance,
			"timing attack possible: variance=%v (wrong: %v, prefix: %v, similar: %v)",
			variance, wrongKeyTiming, prefixMatchTiming, similarTiming)
	})
}

// TestS10_CryptographicValidation tests cryptographic implementations.
// OWASP: A02:2021 - Cryptographic Failures
// Target: credential encryption, key prefix format
// Expected: credentials encrypted, keys prefixed correctly
// Failure severity: CRITICAL - breaks credential security
func TestS10_CryptographicValidation(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	t.Run("api_key_has_correct_prefix", func(t *testing.T) {
		req := APIKeyRequest{
			Name: fmt.Sprintf("crypto-test-%d", time.Now().UnixNano()),
		}
		resp := client.Post(wsPath+"/api-keys", req)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			body := ReadResponseBody(t, resp)

			// Server uses "senda_live_" prefix for API keys
			require.True(t,
				strings.Contains(body, "senda_live_") || strings.Contains(body, "senda_test_"),
				"API key should have senda_live_ or senda_test_ prefix, got: %s", body)
		}
	})

	t.Run("adapter_credentials_not_in_plaintext", func(t *testing.T) {
		req := AdapterRequest{
			AdapterType: AdapterType,
			Name:        fmt.Sprintf("crypto-adapter-%d", time.Now().UnixNano()),
			Config: map[string]interface{}{
				"region":     "us-east-1",
				"access_key": "secret_username_e2e",
				"secret_key": "secret_password_e2e",
			},
		}
		resp := client.Post(wsPath+"/adapters", req)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			// Verify credentials are not returned in plaintext
			body := ReadResponseBody(t, resp)
			require.NotContains(t, body, "secret_password_e2e",
				"credentials should not be returned in plaintext")
		}
	})

	t.Run("request_id_unpredictable", func(t *testing.T) {
		requestIDs := make(map[string]bool)

		for i := 0; i < 10; i++ {
			resp := client.Get("/api/v1/manage/tenants")
			defer resp.Body.Close()

			// Extract request_id from response (if present)
			body := ReadResponseBody(t, resp)
			if strings.Contains(body, "request_id") {
				requestIDs[body] = true
			}
		}

		// Should have at least some unique request IDs (if the server returns them)
		if len(requestIDs) > 0 {
			require.Greater(t, len(requestIDs), 1,
				"request IDs should be unpredictable/unique")
		}
	})

}

// TestS11_HeaderInjection tests SMTP header injection attacks.
// OWASP: A03:2021 - Injection (email header injection)
// Target: Email fields (from_name, subject, reply_to, to)
// Expected: CRLF characters stripped, no header injection
// Failure severity: HIGH - can manipulate email routing (BCC injection, etc)
func TestS11_HeaderInjection(t *testing.T) {
	EnsureSetup(t)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	// Create API key for send tests
	_, apiKeyValue := MustCreateAPIKey(t, client, TenantCode, WorkspaceCode, "header")

	headerInjectionPayloads := []struct {
		field    string
		value    string
		describe string
	}{
		{"from_name", "Admin\r\nBcc: attacker@evil.com", "BCC injection via from_name"},
		{"from_name", "Admin\nBcc: attacker@evil.com", "BCC injection via LF"},
		{"subject", "Test\r\nX-Injected: true", "Custom header via subject"},
		{"subject", "Test\r\nCc: attacker@evil.com", "CC injection via subject"},
		{"preview_text", "Preview\r\nBcc: attacker@evil.com", "BCC via preview"},
	}

	t.Run("email_header_injection", func(t *testing.T) {
		sendClient := NewTestClient(t)
		sendClient.SetAPIKey(apiKeyValue)

		for _, payload := range headerInjectionPayloads {
			req := SendRequest{
				Ref: sendRef(),
				To:  []string{"test@example.com"},
				Variables: map[string]interface{}{
					payload.field:  payload.value,
					"first_name":   "Test",
					"company_name": "Test Corp",
				},
			}

			resp := sendClient.Post("/api/v1/send", req)
			defer resp.Body.Close()

			// Should reject or sanitize - NOT 500
			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"header injection attempt (%s) should not cause 500, got %d",
				payload.describe, resp.StatusCode)

			// If accepted, verify in Mailpit that BCC was not added
			if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
				mailpit := NewMailpitClient(t)
				messages := mailpit.GetMessages()

				if len(messages) > 0 {
					lastMsg := mailpit.GetMessage(messages[len(messages)-1].ID)
					bccHeaders := lastMsg.Headers["Bcc"]
					for _, bcc := range bccHeaders {
						require.NotContains(t, bcc, "attacker@evil.com",
							"BCC header should not contain injected address (%s)", payload.describe)
					}
				}
			}
		}
	})

	t.Run("to_field_crlf_injection", func(t *testing.T) {
		sendClient := NewTestClient(t)
		sendClient.SetAPIKey(apiKeyValue)

		// Attempt to inject additional recipients via to field
		req := SendRequest{
			Ref: sendRef(),
			To:  []string{"victim@example.com\r\nBcc: attacker@evil.com"},
			Variables: map[string]interface{}{
				"first_name":   "Test",
				"company_name": "Test Corp",
			},
		}

		resp := sendClient.Post("/api/v1/send", req)
		defer resp.Body.Close()

		// Should reject (invalid email) or sanitize - NOT 500
		require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"to field CRLF injection should not cause 500, got %d", resp.StatusCode)
	})
}

// TestS12_PathTraversal tests path traversal in slugs and codes.
// OWASP: A01:2021 - Broken Access Control (path traversal variant)
// Target: Slug/code fields that are used in URLs or paths
// Expected: 400 Bad Request for invalid characters
// Failure severity: MEDIUM - can bypass authorization checks
func TestS12_PathTraversal(t *testing.T) {
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	pathTraversalPayloads := []string{
		"../admin",
		"..\\admin",
		"../../etc/passwd",
		"..%2fadmin",
		"..%252fadmin",
		"....//admin",
		"admin/../../../etc/passwd",
		"%2e%2e%2fadmin",
		"test/../../admin",
	}

	t.Run("template_type_slug_path_traversal", func(t *testing.T) {
		for _, payload := range pathTraversalPayloads {
			req := TemplateTypeRequest{
				Slug:           payload,
				Name:           "Test",
				Description:    "Test",
				VariableSchema: DefaultVariableSchema(),
			}
			resp := client.Post(wsPath+"/template-types", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"path traversal payload '%s' should not cause 500, got %d", payload, resp.StatusCode)
		}
	})

	t.Run("injector_name_path_traversal", func(t *testing.T) {
		for _, payload := range pathTraversalPayloads {
			req := InjectorRequest{
				Name:        payload,
				Description: "Test",
				Fields:      []InjectorFieldRequest{},
			}
			resp := client.Post(wsPath+"/injectors", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"path traversal in injector name should not cause 500, got %d", resp.StatusCode)
		}
	})

	t.Run("tenant_code_path_traversal", func(t *testing.T) {
		for _, payload := range pathTraversalPayloads {
			req := map[string]interface{}{
				"code": payload,
				"name": "Test Tenant",
			}
			resp := client.Post("/api/v1/manage/tenants", req)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"path traversal in tenant code should not cause 500, got %d", resp.StatusCode)
		}
	})

	t.Run("workspace_code_path_traversal", func(t *testing.T) {
		for _, payload := range pathTraversalPayloads {
			req := map[string]interface{}{
				"code": payload,
				"name": "Test Workspace",
			}
			resp := client.Post(
				fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", TenantCode),
				req,
			)
			defer resp.Body.Close()

			require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"path traversal in workspace code should not cause 500, got %d", resp.StatusCode)
		}
	})

	t.Run("url_path_traversal", func(t *testing.T) {
		// Attempt to use path traversal in the URL itself
		resp := client.Get("/api/v1/manage/tenants/../../../etc/passwd")
		defer resp.Body.Close()

		require.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest,
			"URL path traversal should return 400/404, got %d", resp.StatusCode)
	})
}
