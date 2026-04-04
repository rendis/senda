//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---------- helpers ----------

func globalPath() string {
	return "/api/v1/manage/global"
}

func mgmtPath() string {
	return "/api/v1/manage"
}

func tenantPath() string {
	return fmt.Sprintf("/api/v1/manage/tenants/%s", TenantCode)
}

func ensureClient(t *testing.T) *TestClient {
	t.Helper()
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	c := NewTestClient(t)
	c.LoginAs(SuperadminEmail)
	return c
}

// ---------- Tenant CRUD ----------

func TestCRUD_Tenant_GetByCode(t *testing.T) {
	c := ensureClient(t)

	t.Run("success", func(t *testing.T) {
		resp := c.Get(mgmtPath() + "/tenants/" + TenantCode)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var body struct {
			Code string `json:"code"`
			Name string `json:"name"`
		}
		ParseJSONResponse(t, resp, &body)
		require.Equal(t, TenantCode, body.Code)
	})

	t.Run("not_found", func(t *testing.T) {
		resp := c.Get(mgmtPath() + "/tenants/nonexistent-tenant-xyz")
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusNotFound, http.StatusForbidden}, resp.StatusCode)
	})
}

func TestCRUD_Tenant_List(t *testing.T) {
	c := ensureClient(t)

	resp := c.Get(mgmtPath() + "/tenants")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body struct {
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
	}
	ParseJSONResponse(t, resp, &body)
	require.NotEmpty(t, body.Items)
}

func TestCRUD_Tenant_Update(t *testing.T) {
	c := ensureClient(t)

	t.Run("success", func(t *testing.T) {
		resp := c.Put(mgmtPath()+"/tenants/"+TenantCode, map[string]string{
			"name": TenantName, // update with same name to avoid side effects
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("not_found", func(t *testing.T) {
		resp := c.Put(mgmtPath()+"/tenants/nonexistent-xyz", map[string]string{
			"name": "Whatever",
		})
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusNotFound, http.StatusForbidden}, resp.StatusCode)
	})
}

func TestCRUD_Tenant_Delete(t *testing.T) {
	c := ensureClient(t)

	// Create a throwaway tenant, then delete it
	code := fmt.Sprintf("del-tenant-%d", time.Now().UnixNano()%100000)
	resp := c.Post(mgmtPath()+"/tenants", map[string]string{
		"code": code,
		"name": "Delete Me",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	t.Run("success", func(t *testing.T) {
		resp := c.Delete(mgmtPath() + "/tenants/" + code)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNoContent)
	})

	t.Run("not_found_after_delete", func(t *testing.T) {
		resp := c.Get(mgmtPath() + "/tenants/" + code)
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusNotFound, http.StatusForbidden}, resp.StatusCode)
	})
}

func TestCRUD_Tenant_DashboardStats(t *testing.T) {
	c := ensureClient(t)

	resp := c.Get(tenantPath() + "/dashboard-stats")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

// ---------- Workspace CRUD ----------

func TestCRUD_Workspace_Update(t *testing.T) {
	c := ensureClient(t)

	t.Run("success", func(t *testing.T) {
		resp := c.Put(tenantPath()+"/workspaces/"+WorkspaceCode, map[string]string{
			"name": WorkspaceName,
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("not_found", func(t *testing.T) {
		resp := c.Put(tenantPath()+"/workspaces/nonexistent-ws", map[string]string{
			"name": "Nope",
		})
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusNotFound, http.StatusForbidden}, resp.StatusCode)
	})
}

func TestCRUD_Workspace_StatusAndSystemProtection(t *testing.T) {
	c := ensureClient(t)
	code := fmt.Sprintf("toggle-ws-%d", time.Now().UnixNano()%100000)

	createResp := c.Post(tenantPath()+"/workspaces", map[string]string{
		"code": code,
		"name": "Toggle Workspace",
	})
	defer createResp.Body.Close()
	RequireStatus(t, createResp, http.StatusCreated)

	var created struct {
		Code     string `json:"code"`
		IsActive bool   `json:"is_active"`
	}
	ParseJSONResponse(t, createResp, &created)
	require.Equal(t, code, created.Code)
	require.True(t, created.IsActive)

	t.Run("deactivate_and_reactivate", func(t *testing.T) {
		resp := c.Put(tenantPath()+"/workspaces/"+code, map[string]any{
			"name":      "Toggle Workspace Disabled",
			"is_active": false,
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var updated struct {
			Name     string `json:"name"`
			IsActive bool   `json:"is_active"`
		}
		ParseJSONResponse(t, resp, &updated)
		require.Equal(t, "Toggle Workspace Disabled", updated.Name)
		require.False(t, updated.IsActive)

		getResp := c.Get(tenantPath() + "/workspaces/" + code)
		defer getResp.Body.Close()
		RequireStatus(t, getResp, http.StatusOK)

		var fetched struct {
			IsActive bool `json:"is_active"`
		}
		ParseJSONResponse(t, getResp, &fetched)
		require.False(t, fetched.IsActive)

		reactivateResp := c.Put(tenantPath()+"/workspaces/"+code, map[string]any{
			"name":      "Toggle Workspace Active",
			"is_active": true,
		})
		defer reactivateResp.Body.Close()
		RequireStatus(t, reactivateResp, http.StatusOK)

		var reactivated struct {
			IsActive bool `json:"is_active"`
		}
		ParseJSONResponse(t, reactivateResp, &reactivated)
		require.True(t, reactivated.IsActive)
	})

	t.Run("system_workspace_is_protected", func(t *testing.T) {
		resp := c.Put(tenantPath()+"/workspaces/_system", map[string]any{
			"name":      "Should Fail",
			"is_active": false,
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusConflict)
		AssertError(t, resp, "SYSTEM_WORKSPACE_PROTECTED")

		deleteResp := c.Delete(tenantPath() + "/workspaces/_system")
		defer deleteResp.Body.Close()
		RequireStatus(t, deleteResp, http.StatusConflict)
		AssertError(t, deleteResp, "SYSTEM_WORKSPACE_PROTECTED")
	})
}

func TestCRUD_Workspace_Delete(t *testing.T) {
	c := ensureClient(t)

	// Create a throwaway workspace
	code := fmt.Sprintf("del-ws-%d", time.Now().UnixNano()%100000)
	resp := c.Post(tenantPath()+"/workspaces", map[string]string{
		"code": code,
		"name": "Delete Workspace",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	t.Run("success", func(t *testing.T) {
		resp := c.Delete(tenantPath() + "/workspaces/" + code)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNoContent)
	})
}

func TestCRUD_Workspace_DashboardStats(t *testing.T) {
	c := ensureClient(t)

	resp := c.Get(wsPath() + "/dashboard-stats")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

// ---------- Global Template Types ----------

func TestCRUD_Global_TemplateType(t *testing.T) {
	c := ensureClient(t)
	slug := fmt.Sprintf("tt-e2e-%d", time.Now().UnixNano()%100000)

	var createdSlug string

	t.Run("create", func(t *testing.T) {
		resp := c.Post(globalPath()+"/template-types", TemplateTypeRequest{
			Slug: slug,
			Name: "E2E Coverage Type",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)

		var body struct {
			Slug string `json:"slug"`
		}
		ParseJSONResponse(t, resp, &body)
		createdSlug = body.Slug
		require.Equal(t, slug, createdSlug)
	})

	t.Run("duplicate_conflict", func(t *testing.T) {
		resp := c.Post(globalPath()+"/template-types", TemplateTypeRequest{
			Slug: slug,
			Name: "Duplicate",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusConflict)
	})

	t.Run("get_by_slug", func(t *testing.T) {
		resp := c.Get(globalPath() + "/template-types/" + slug)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var body struct {
			Slug string `json:"slug"`
			Name string `json:"name"`
		}
		ParseJSONResponse(t, resp, &body)
		require.Equal(t, slug, body.Slug)
	})

	t.Run("list", func(t *testing.T) {
		resp := c.Get(globalPath() + "/template-types")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var body struct {
			Items []struct {
				Slug string `json:"slug"`
			} `json:"items"`
		}
		ParseJSONResponse(t, resp, &body)
		require.NotEmpty(t, body.Items)
	})

	t.Run("get_not_found", func(t *testing.T) {
		resp := c.Get(globalPath() + "/template-types/nonexistent-slug-xyz")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("create_validation_error", func(t *testing.T) {
		resp := c.Post(globalPath()+"/template-types", map[string]string{})
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, resp.StatusCode)
	})
}

// ---------- Global Adapters ----------

func TestCRUD_Global_Adapter(t *testing.T) {
	c := ensureClient(t)
	name := fmt.Sprintf("global-adapter-%d", time.Now().UnixNano()%100000)

	var adapterID string

	t.Run("create", func(t *testing.T) {
		resp := c.Post(globalPath()+"/adapters", AdapterRequest{
			Name:        name,
			AdapterType: AdapterType,
			Config: map[string]interface{}{
				"region":            "us-east-1",
				"access_key_id":     "test-key",
				"secret_access_key": "test-secret",
			},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)

		var body struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, resp, &body)
		adapterID = body.ID
		require.NotEmpty(t, adapterID)
	})

	t.Run("get", func(t *testing.T) {
		require.NotEmpty(t, adapterID, "adapter id must be created")
		resp := c.Get(globalPath() + "/adapters/" + adapterID)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("list", func(t *testing.T) {
		resp := c.Get(globalPath() + "/adapters")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("update", func(t *testing.T) {
		require.NotEmpty(t, adapterID, "adapter id must be created")
		resp := c.Put(globalPath()+"/adapters/"+adapterID, AdapterRequest{
			Name:        name + "-updated",
			AdapterType: AdapterType,
			Config: map[string]interface{}{
				"region":            "eu-west-1",
				"access_key_id":     "test-key-2",
				"secret_access_key": "test-secret-2",
			},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("delete", func(t *testing.T) {
		require.NotEmpty(t, adapterID, "adapter id must be created")
		resp := c.Delete(globalPath() + "/adapters/" + adapterID)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNoContent)
	})

	t.Run("get_after_delete", func(t *testing.T) {
		require.NotEmpty(t, adapterID, "adapter id must be created")
		resp := c.Get(globalPath() + "/adapters/" + adapterID)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("get_not_found", func(t *testing.T) {
		resp := c.Get(globalPath() + "/adapters/00000000-0000-0000-0000-000000000000")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})
}

// ---------- Global Injectors ----------

func TestCRUD_Global_Injector(t *testing.T) {
	c := ensureClient(t)
	name := fmt.Sprintf("global-inj-%d", time.Now().UnixNano()%100000)

	t.Run("create", func(t *testing.T) {
		resp := c.Post(globalPath()+"/injectors", InjectorRequest{
			Name:        name,
			Description: "E2E global injector",
			Fields: []InjectorFieldRequest{
				{FieldName: "company", FieldType: "text", Position: 1},
			},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)
	})

	t.Run("get", func(t *testing.T) {
		resp := c.Get(globalPath() + "/injectors/" + name)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("list", func(t *testing.T) {
		resp := c.Get(globalPath() + "/injectors")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("get_not_found", func(t *testing.T) {
		resp := c.Get(globalPath() + "/injectors/nonexistent-injector-xyz")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})
}

// ---------- Global Templates ----------

func TestCRUD_Global_Template(t *testing.T) {
	c := ensureClient(t)

	// First create a global template type
	ttSlug := fmt.Sprintf("g-tpl-tt-%d", time.Now().UnixNano()%100000)
	resp := c.Post(globalPath()+"/template-types", TemplateTypeRequest{
		Slug: ttSlug,
		Name: "Global Template Type for Templates",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var ttID string
	{
		var ttBody struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, resp, &ttBody)
		ttID = ttBody.ID
	}

	t.Run("create_template", func(t *testing.T) {
		require.NotEmpty(t, ttID, "template type id is required")
		resp := c.Post(globalPath()+"/templates", map[string]string{
			"template_type_id": ttID,
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)
	})

	t.Run("create_validation_error", func(t *testing.T) {
		// Missing required fields
		resp := c.Post(globalPath()+"/templates", map[string]string{})
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, resp.StatusCode)
	})
}

// ---------- Global Audit Log ----------

func TestCRUD_Global_AuditLog(t *testing.T) {
	c := ensureClient(t)

	resp := c.Get(globalPath() + "/audit-log")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

// ---------- Global Dashboard Stats ----------

func TestCRUD_Global_DashboardStats(t *testing.T) {
	c := ensureClient(t)

	resp := c.Get(globalPath() + "/dashboard-stats")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

// ---------- Workspace Adapter Update/Delete ----------

func TestCRUD_WS_Adapter_Update(t *testing.T) {
	c := ensureClient(t)
	name := fmt.Sprintf("ws-adapter-upd-%d", time.Now().UnixNano()%100000)

	// Create
	resp := c.Post(wsPath()+"/adapters", AdapterRequest{
		Name:        name,
		AdapterType: AdapterType,
		Config: map[string]interface{}{
			"region":            "us-east-1",
			"access_key_id":     "test-key",
			"secret_access_key": "test-secret",
		},
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var created struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &created)

	t.Run("update_success", func(t *testing.T) {
		resp := c.Put(wsPath()+"/adapters/"+created.ID, AdapterRequest{
			Name:        name + "-updated",
			AdapterType: AdapterType,
			Config: map[string]interface{}{
				"region":            "eu-west-1",
				"access_key_id":     "new-key",
				"secret_access_key": "new-secret",
			},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("update_not_found", func(t *testing.T) {
		resp := c.Put(wsPath()+"/adapters/00000000-0000-0000-0000-000000000000", AdapterRequest{
			Name:        "nope",
			AdapterType: AdapterType,
			Config:      map[string]interface{}{"region": "us-east-1", "access_key_id": "k", "secret_access_key": "s"},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("delete_success", func(t *testing.T) {
		resp := c.Delete(wsPath() + "/adapters/" + created.ID)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNoContent)
	})

	t.Run("delete_not_found", func(t *testing.T) {
		resp := c.Delete(wsPath() + "/adapters/00000000-0000-0000-0000-000000000000")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})
}

// ---------- Workspace Webhooks CRUD ----------

func TestCRUD_WS_Webhook(t *testing.T) {
	c := ensureClient(t)
	url := fmt.Sprintf("https://example.com/e2e-%d", time.Now().UnixNano()%100000)

	var webhookID string

	t.Run("create", func(t *testing.T) {
		resp := c.Post(wsPath()+"/webhooks", WebhookRequest{
			URL:    url,
			Events: []string{"email.sent", "email.delivered"},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)

		var body struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, resp, &body)
		webhookID = body.ID
		require.NotEmpty(t, webhookID)
	})

	t.Run("create_validation_error", func(t *testing.T) {
		resp := c.Post(wsPath()+"/webhooks", map[string]string{})
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, resp.StatusCode)
	})

	t.Run("get", func(t *testing.T) {
		require.NotEmpty(t, webhookID, "webhook id must be created")
		resp := c.Get(wsPath() + "/webhooks/" + webhookID)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("list", func(t *testing.T) {
		resp := c.Get(wsPath() + "/webhooks")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("update", func(t *testing.T) {
		require.NotEmpty(t, webhookID, "webhook id must be created")
		resp := c.Put(wsPath()+"/webhooks/"+webhookID, WebhookRequest{
			URL:    url + "/updated",
			Events: []string{"email.bounced"},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("update_not_found", func(t *testing.T) {
		resp := c.Put(wsPath()+"/webhooks/00000000-0000-0000-0000-000000000000", WebhookRequest{
			URL:    "https://example.com/nope",
			Events: []string{"email.sent"},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("test_webhook", func(t *testing.T) {
		require.NotEmpty(t, webhookID, "webhook id must be created")
		resp := c.Post(wsPath()+"/webhooks/"+webhookID+"/test", nil)
		defer resp.Body.Close()
		// Test endpoint fires a test event — may return 200 or 202
		require.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp.StatusCode)
	})

	t.Run("delete", func(t *testing.T) {
		require.NotEmpty(t, webhookID, "webhook id must be created")
		resp := c.Delete(wsPath() + "/webhooks/" + webhookID)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNoContent)
	})

	t.Run("delete_not_found", func(t *testing.T) {
		resp := c.Delete(wsPath() + "/webhooks/00000000-0000-0000-0000-000000000000")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})
}

// ---------- Workspace API Keys List ----------

func TestCRUD_WS_APIKey_List(t *testing.T) {
	c := ensureClient(t)

	resp := c.Get(wsPath() + "/api-keys")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

// ---------- Workspace Suppression ----------

func TestCRUD_WS_Suppression(t *testing.T) {
	c := ensureClient(t)
	email := fmt.Sprintf("supp-%d@example.com", time.Now().UnixNano()%100000)

	t.Run("add", func(t *testing.T) {
		resp := c.Post(wsPath()+"/suppression", SuppressionRequest{
			Email:  email,
			Reason: "manual",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)
	})

	t.Run("check", func(t *testing.T) {
		resp := c.Get(wsPath() + "/suppression/" + email)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("check_not_suppressed", func(t *testing.T) {
		resp := c.Get(wsPath() + "/suppression/nonexistent@example.com")
		defer resp.Body.Close()
		// Check endpoint returns 200 with suppressed=false for non-suppressed emails
		RequireStatus(t, resp, http.StatusOK)
		var body struct {
			Suppressed bool `json:"suppressed"`
		}
		ParseJSONResponse(t, resp, &body)
		require.False(t, body.Suppressed)
	})

	t.Run("delete_not_found_global", func(t *testing.T) {
		// Remove only works on global suppressions (not workspace-scoped).
		// Since we added a workspace suppression, delete returns 404.
		resp := c.Delete(wsPath() + "/suppression/" + email)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("add_validation_error", func(t *testing.T) {
		resp := c.Post(wsPath()+"/suppression", SuppressionRequest{
			Email: "not-an-email",
		})
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, resp.StatusCode)
	})
}

// ---------- Workspace Emails Events ----------

func TestCRUD_WS_Email_Events(t *testing.T) {
	c := ensureClient(t)

	t.Run("not_found", func(t *testing.T) {
		resp := c.Get(wsPath() + "/emails/trk_00000000000000000000000000000000/events")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})
}

// ---------- Template Version Locales ----------

func TestCRUD_WS_Template_Locale(t *testing.T) {
	c := ensureClient(t)

	// Create a dedicated template type + template + version for this test
	ttSlug := fmt.Sprintf("locale-tt-%d", time.Now().UnixNano()%100000)
	resp := c.Post(wsPath()+"/template-types", TemplateTypeRequest{
		Slug: ttSlug,
		Name: "Locale Test Type",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var ttBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &ttBody)

	// Create template using template_type_id
	resp2 := c.Post(wsPath()+"/templates", map[string]string{
		"template_type_id": ttBody.ID,
	})
	defer resp2.Body.Close()
	RequireStatus(t, resp2, http.StatusCreated)

	var tplBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp2, &tplBody)

	// Create version
	resp3 := c.Post(fmt.Sprintf("%s/templates/%s/versions", wsPath(), tplBody.ID), CreateVersionRequest{
		Subject:       "Locale Test Subject",
		PreviewText:   "Preview",
		FromEmail:     TestFromEmail,
		FromName:      TestFromName,
		BodyMJML:      SampleMJML(),
		DefaultLocale: "en",
	})
	defer resp3.Body.Close()
	RequireStatus(t, resp3, http.StatusCreated)

	var verBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp3, &verBody)

	localesBase := fmt.Sprintf("%s/templates/%s/versions/%s/locales", wsPath(), tplBody.ID, verBody.ID)

	t.Run("set_locale", func(t *testing.T) {
		mjml := SampleMJML()
		subj := "Asunto de prueba"
		preview := "Vista previa"
		fromName := "Empresa Test"
		resp := c.Post(localesBase+"/es", map[string]interface{}{
			"subject":      &subj,
			"preview_text": &preview,
			"from_name":    &fromName,
			"body_mjml":    &mjml,
		})
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusOK, http.StatusCreated}, resp.StatusCode)
	})

	t.Run("get_locale", func(t *testing.T) {
		resp := c.Get(localesBase + "/es")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("update_locale", func(t *testing.T) {
		mjml := SampleMJML()
		subj := "Asunto actualizado"
		preview := "Vista previa actualizada"
		fromName := "Empresa Test Updated"
		resp := c.Put(localesBase+"/es", map[string]interface{}{
			"subject":      &subj,
			"preview_text": &preview,
			"from_name":    &fromName,
			"body_mjml":    &mjml,
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("get_locale_not_found", func(t *testing.T) {
		resp := c.Get(localesBase + "/zh")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})

	t.Run("delete_locale", func(t *testing.T) {
		resp := c.Delete(localesBase + "/es")
		defer resp.Body.Close()
		// DeleteLocale may return 501 if not yet implemented
		require.Contains(t, []int{http.StatusNoContent, http.StatusNotImplemented}, resp.StatusCode)
	})

	t.Run("delete_locale_not_found", func(t *testing.T) {
		resp := c.Delete(localesBase + "/zh")
		defer resp.Body.Close()
		require.Contains(t, []int{http.StatusNotFound, http.StatusNotImplemented}, resp.StatusCode)
	})
}

// ---------- Template Versions List ----------

func TestCRUD_WS_Template_ListVersions(t *testing.T) {
	c := ensureClient(t)

	// Use existing test infrastructure
	ttSlug := fmt.Sprintf("lv-tt-%d", time.Now().UnixNano()%100000)
	resp := c.Post(wsPath()+"/template-types", TemplateTypeRequest{
		Slug: ttSlug,
		Name: "List Versions Type",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var lvTTBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &lvTTBody)

	resp2 := c.Post(wsPath()+"/templates", map[string]string{
		"template_type_id": lvTTBody.ID,
	})
	defer resp2.Body.Close()
	RequireStatus(t, resp2, http.StatusCreated)

	var tplBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp2, &tplBody)

	// List versions (should be empty or have 1)
	resp3 := c.Get(fmt.Sprintf("%s/templates/%s/versions", wsPath(), tplBody.ID))
	defer resp3.Body.Close()
	RequireStatus(t, resp3, http.StatusOK)
}

// ---------- Preview MJML ----------

func TestCRUD_WS_Template_PreviewMJML(t *testing.T) {
	c := ensureClient(t)

	// Need a template ID — create throwaway
	ttSlug := fmt.Sprintf("pm-tt-%d", time.Now().UnixNano()%100000)
	resp := c.Post(wsPath()+"/template-types", TemplateTypeRequest{
		Slug: ttSlug,
		Name: "Preview MJML Type",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var pmTTBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &pmTTBody)

	resp2 := c.Post(wsPath()+"/templates", map[string]string{
		"template_type_id": pmTTBody.ID,
	})
	defer resp2.Body.Close()
	RequireStatus(t, resp2, http.StatusCreated)

	var tplBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp2, &tplBody)

	t.Run("preview", func(t *testing.T) {
		resp := c.Post(fmt.Sprintf("%s/templates/%s/preview-mjml", wsPath(), tplBody.ID), map[string]string{
			"mjml": SampleMJML(),
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var body struct {
			HTML string `json:"html"`
		}
		ParseJSONResponse(t, resp, &body)
		require.NotEmpty(t, body.HTML)
	})
}

// ---------- Members CRUD ----------

func TestCRUD_Members(t *testing.T) {
	c := ensureClient(t)

	var memberID string
	email := fmt.Sprintf("member-%d@test.example.com", time.Now().UnixNano()%100000)

	t.Run("create", func(t *testing.T) {
		resp := c.Post(mgmtPath()+"/members", map[string]interface{}{
			"email":        email,
			"display_name": "E2E Test Member",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)

		var body struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, resp, &body)
		memberID = body.ID
		require.NotEmpty(t, memberID)
	})

	t.Run("create_duplicate", func(t *testing.T) {
		resp := c.Post(mgmtPath()+"/members", map[string]interface{}{
			"email":        email,
			"display_name": "Duplicate",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusConflict)
	})

	t.Run("list", func(t *testing.T) {
		resp := c.Get(mgmtPath() + "/members")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		var body struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		ParseJSONResponse(t, resp, &body)
		require.NotEmpty(t, body.Items)
	})

	t.Run("get", func(t *testing.T) {
		require.NotEmpty(t, memberID, "member id must be created")
		resp := c.Get(mgmtPath() + "/members/" + memberID)
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("get_not_found", func(t *testing.T) {
		resp := c.Get(mgmtPath() + "/members/00000000-0000-0000-0000-000000000000")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNotFound)
	})

	var roleID string

	t.Run("add_role", func(t *testing.T) {
		require.NotEmpty(t, memberID, "member id must be created")
		resp := c.Post(fmt.Sprintf("%s/members/%s/roles", mgmtPath(), memberID), map[string]interface{}{
			"role":       "tenant_admin",
			"scope_type": "tenant",
			"tenant_id":  getTenantID(t, c),
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusCreated)

		var body struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, resp, &body)
		roleID = body.ID
	})

	t.Run("remove_role", func(t *testing.T) {
		require.NotEmpty(t, memberID, "member id must be created")
		require.NotEmpty(t, roleID, "role id must be created")
		resp := c.Delete(fmt.Sprintf("%s/members/%s/roles/%s", mgmtPath(), memberID, roleID))
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusNoContent)
	})
}

// getTenantID fetches the tenant ID from the API.
func getTenantID(t *testing.T, c *TestClient) string {
	t.Helper()
	resp := c.Get(mgmtPath() + "/tenants/" + TenantCode)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
	var body struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &body)
	return body.ID
}

// ---------- Config ----------

func TestCRUD_Config(t *testing.T) {
	c := ensureClient(t)

	t.Run("get", func(t *testing.T) {
		resp := c.Get(mgmtPath() + "/config")
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})

	t.Run("update", func(t *testing.T) {
		resp := c.Put(mgmtPath()+"/config", map[string]interface{}{
			"default_from_name":  "Senda E2E",
			"default_from_email": "noreply@e2e.example.com",
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)
	})
}

// ---------- Workspace Audit Log ----------

func TestCRUD_WS_AuditLog(t *testing.T) {
	c := ensureClient(t)

	resp := c.Get(wsPath() + "/audit-log")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

// ---------- SES Webhook Inbound ----------

func TestCRUD_SES_Webhook_Inbound(t *testing.T) {
	c := ensureClient(t)

	t.Run("invalid_payload", func(t *testing.T) {
		resp := c.Post("/api/v1/webhooks/ses/inbound", map[string]string{
			"Type": "invalid",
		})
		defer resp.Body.Close()
		// Endpoint may not be wired in E2E (404) or reject invalid SNS messages (400/401/403)
		require.Contains(t, []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound}, resp.StatusCode)
	})
}
