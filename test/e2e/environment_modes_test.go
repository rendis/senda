//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type environmentWorkspaceResponse struct {
	ID                     string   `json:"id"`
	LogicalWorkspaceID     string   `json:"logical_workspace_id"`
	TenantID               string   `json:"tenant_id"`
	Code                   string   `json:"code"`
	Name                   string   `json:"name"`
	Environment            string   `json:"environment"`
	IsActive               bool     `json:"is_active"`
	DefaultLocale          *string  `json:"default_locale"`
	TestRecipientMode      string   `json:"test_recipient_mode"`
	TestRecipientAddresses []string `json:"test_recipient_addresses"`
}

type environmentWorkspaceListResponse struct {
	Items []environmentWorkspaceResponse `json:"items"`
}

type environmentSendResponse struct {
	Status           string `json:"status"`
	TemplateResolved string `json:"template_resolved"`
	TrackingIDs      []struct {
		To         string `json:"to"`
		TrackingID string `json:"tracking_id"`
		Status     string `json:"status"`
		Error      string `json:"error"`
	} `json:"tracking_ids"`
}

type environmentEmailListResponse struct {
	Items []struct {
		TrackingID       string `json:"tracking_id"`
		RecipientEmail   string `json:"recipient_email"`
		TemplateTypeSlug string `json:"template_type_slug"`
		Status           string `json:"status"`
	} `json:"items"`
}

type environmentTemplateSetup struct {
	TemplateTypeID string
	TemplateID     string
	VersionID      string
	AdapterID      string
}

type environmentTemplatePolicy struct {
	Mode      *string
	Addresses []string
}

func managementTenantWorkspacesPath(tenantCode string) string {
	return fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", tenantCode)
}

func managementWorkspacePath(tenantCode, workspaceCode string) string {
	return fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", tenantCode, workspaceCode)
}

func managementEnvironmentTenantWorkspacesPath(environment, tenantCode string) string {
	return fmt.Sprintf("/api/v1/manage/environments/%s/tenants/%s/workspaces", environment, tenantCode)
}

func managementEnvironmentWorkspacePath(environment, tenantCode, workspaceCode string) string {
	return fmt.Sprintf("/api/v1/manage/environments/%s/tenants/%s/workspaces/%s", environment, tenantCode, workspaceCode)
}

func ensureEnvironmentBaseline(t *testing.T) {
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
}

func mustCreateLogicalWorkspacePair(t *testing.T, client *TestClient, code, name string) environmentWorkspaceResponse {
	t.Helper()

	resp := client.Post(managementTenantWorkspacesPath(TenantCode), map[string]any{
		"code": code,
		"name": name,
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var body environmentWorkspaceResponse
	ParseJSONResponse(t, resp, &body)
	return body
}

func mustGetWorkspace(t *testing.T, client *TestClient, path string) environmentWorkspaceResponse {
	t.Helper()

	resp := client.Get(path)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body environmentWorkspaceResponse
	ParseJSONResponse(t, resp, &body)
	return body
}

func mustFindWorkspaceInList(t *testing.T, client *TestClient, path, code string) environmentWorkspaceResponse {
	t.Helper()

	resp := client.Get(path)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body environmentWorkspaceListResponse
	ParseJSONResponse(t, resp, &body)
	for _, item := range body.Items {
		if item.Code == code {
			return item
		}
	}

	t.Fatalf("workspace %q not found in %s", code, path)
	return environmentWorkspaceResponse{}
}

func mustCreateAPIKeyAtWorkspacePath(t *testing.T, client *TestClient, workspacePath, suffix string) string {
	t.Helper()

	resp := client.Post(workspacePath+"/api-keys", APIKeyRequest{
		Name: fmt.Sprintf("%s%s-%d", APIKeyNamePrefix, suffix, time.Now().UnixNano()),
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var body struct {
		Key   string `json:"key"`
		Token string `json:"token"`
	}
	ParseJSONResponse(t, resp, &body)
	if body.Key != "" {
		return body.Key
	}
	require.NotEmpty(t, body.Token)
	return body.Token
}

func mustCreateEnvironmentTemplateSetup(t *testing.T, client *TestClient, workspacePath, slug, name string, policy *environmentTemplatePolicy) environmentTemplateSetup {
	t.Helper()

	adapterResp := client.Post(workspacePath+"/adapters", AdapterRequest{
		AdapterType:        AdapterType,
		Name:               fmt.Sprintf("%s-adapter-%d", slug, time.Now().UnixNano()),
		RateLimitPerSecond: 100,
		Config: map[string]interface{}{
			"region":     "us-east-1",
			"access_key": "test",
			"secret_key": "test",
		},
	})
	defer adapterResp.Body.Close()
	RequireStatus(t, adapterResp, http.StatusCreated)

	var adapterBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, adapterResp, &adapterBody)
	require.NotEmpty(t, adapterBody.ID)
	EnsureDefaultAdapterIdentity(t, adapterBody.ID, TestFromEmail)

	templateTypeBody := map[string]any{
		"slug":            slug,
		"name":            name,
		"description":     fmt.Sprintf("Template type for %s", slug),
		"adapter_id":      adapterBody.ID,
		"variable_schema": DefaultVariableSchema(),
	}
	if policy != nil {
		if policy.Mode != nil {
			templateTypeBody["test_recipient_mode"] = *policy.Mode
		}
		if policy.Addresses != nil {
			templateTypeBody["test_recipient_addresses"] = policy.Addresses
		}
	}

	templateTypeResp := client.Post(workspacePath+"/template-types", templateTypeBody)
	defer templateTypeResp.Body.Close()
	RequireStatus(t, templateTypeResp, http.StatusCreated)

	var templateType struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, templateTypeResp, &templateType)
	require.NotEmpty(t, templateType.ID)

	templateResp := client.Post(workspacePath+"/templates", map[string]any{
		"template_type_id": templateType.ID,
	})
	defer templateResp.Body.Close()
	RequireStatus(t, templateResp, http.StatusCreated)

	var templateBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, templateResp, &templateBody)
	require.NotEmpty(t, templateBody.ID)

	versionResp := client.Post(fmt.Sprintf("%s/templates/%s/versions", workspacePath, templateBody.ID), CreateVersionRequest{
		Subject:       fmt.Sprintf("Subject %s", slug),
		PreviewText:   fmt.Sprintf("Preview %s", slug),
		FromEmail:     TestFromEmail,
		FromName:      TestFromName,
		BodyMJML:      SampleMJML(),
		DefaultLocale: "en",
	})
	defer versionResp.Body.Close()
	RequireStatus(t, versionResp, http.StatusCreated)

	var versionBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, versionResp, &versionBody)
	require.NotEmpty(t, versionBody.ID)

	publishResp := client.Post(fmt.Sprintf("%s/templates/%s/versions/%s/publish", workspacePath, templateBody.ID, versionBody.ID), nil)
	defer publishResp.Body.Close()
	require.Equal(t, http.StatusNoContent, publishResp.StatusCode, "publish should succeed: %s", ReadResponseBody(t, publishResp))

	return environmentTemplateSetup{
		TemplateTypeID: templateType.ID,
		TemplateID:     templateBody.ID,
		VersionID:      versionBody.ID,
		AdapterID:      adapterBody.ID,
	}
}

func mustConfigureWorkspaceTestRecipients(t *testing.T, client *TestClient, workspacePath, mode string, addresses []string) {
	t.Helper()

	resp := client.Put(workspacePath, map[string]any{
		"test_recipient_mode":      mode,
		"test_recipient_addresses": addresses,
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

func mustSendInEnvironment(t *testing.T, apiKey, ref string, recipients ...string) environmentSendResponse {
	t.Helper()

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKey)

	resp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: ref,
		To:  recipients,
		Variables: map[string]interface{}{
			"first_name":   "Env",
			"company_name": "Senda",
		},
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusAccepted)

	var body environmentSendResponse
	ParseJSONResponse(t, resp, &body)
	require.Equal(t, "accepted", body.Status)
	require.NotEmpty(t, body.TrackingIDs)
	return body
}

func mustWaitForEmailStatusInEnvironment(t *testing.T, client *TestClient, environment, tenantCode, workspaceCode, trackingID, expectedStatus string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		path := managementWorkspacePath(tenantCode, workspaceCode) + "/emails/" + trackingID
		if environment != "" {
			path = managementEnvironmentWorkspacePath(environment, tenantCode, workspaceCode) + "/emails/" + trackingID
		}

		resp := client.Get(path)
		if resp.StatusCode == http.StatusOK {
			var body struct {
				Status string `json:"status"`
			}
			ParseJSONResponse(t, resp, &body)
			resp.Body.Close()
			if body.Status == expectedStatus {
				return
			}
		} else {
			resp.Body.Close()
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for %s email %s to reach %s", environment, trackingID, expectedStatus)
}

func mustListEmailsInEnvironment(t *testing.T, client *TestClient, environment, tenantCode, workspaceCode string) environmentEmailListResponse {
	t.Helper()

	path := managementWorkspacePath(tenantCode, workspaceCode) + "/emails"
	if environment != "" {
		path = managementEnvironmentWorkspacePath(environment, tenantCode, workspaceCode) + "/emails"
	}

	resp := client.Get(path)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body environmentEmailListResponse
	ParseJSONResponse(t, resp, &body)
	return body
}

func mustListDataPlaneEmails(t *testing.T, apiKey, query string) environmentEmailListResponse {
	t.Helper()

	client := NewTestClient(t)
	client.SetAPIKey(apiKey)

	path := "/api/v1/emails"
	if query != "" {
		path += "?" + query
	}
	resp := client.Get(path)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body environmentEmailListResponse
	ParseJSONResponse(t, resp, &body)
	return body
}

func TestEnv01_WorkspaceLogicalPair_SharedAndIsolatedManagement(t *testing.T) {
	ensureEnvironmentBaseline(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	code := fmt.Sprintf("envpair-%d", time.Now().UnixNano()%1000000)
	name := "Environment Pair Workspace"
	created := mustCreateLogicalWorkspacePair(t, client, code, name)

	require.Equal(t, code, created.Code)
	require.Equal(t, name, created.Name)
	require.Equal(t, "prod", created.Environment)
	require.NotEmpty(t, created.LogicalWorkspaceID)
	require.Equal(t, "replace", created.TestRecipientMode)
	require.Empty(t, created.TestRecipientAddresses)

	prodListItem := mustFindWorkspaceInList(t, client, managementTenantWorkspacesPath(TenantCode), code)
	testListItem := mustFindWorkspaceInList(t, client, managementEnvironmentTenantWorkspacesPath("test", TenantCode), code)
	require.Equal(t, "prod", prodListItem.Environment)
	require.Equal(t, "test", testListItem.Environment)
	require.Equal(t, created.LogicalWorkspaceID, prodListItem.LogicalWorkspaceID)
	require.Equal(t, created.LogicalWorkspaceID, testListItem.LogicalWorkspaceID)
	require.NotEqual(t, prodListItem.ID, testListItem.ID)

	newCode := code + "-renamed"
	newName := "Environment Pair Workspace Renamed"
	updateResp := client.Put(managementWorkspacePath(TenantCode, code), map[string]any{
		"code": newCode,
		"name": newName,
	})
	defer updateResp.Body.Close()
	RequireStatus(t, updateResp, http.StatusOK)

	prodWorkspace := mustGetWorkspace(t, client, managementWorkspacePath(TenantCode, newCode))
	testWorkspace := mustGetWorkspace(t, client, managementEnvironmentWorkspacePath("test", TenantCode, newCode))
	require.Equal(t, newCode, prodWorkspace.Code)
	require.Equal(t, newCode, testWorkspace.Code)
	require.Equal(t, newName, prodWorkspace.Name)
	require.Equal(t, newName, testWorkspace.Name)
	require.Equal(t, prodWorkspace.LogicalWorkspaceID, testWorkspace.LogicalWorkspaceID)

	oldProdResp := client.Get(managementWorkspacePath(TenantCode, code))
	defer oldProdResp.Body.Close()
	require.Equal(t, http.StatusForbidden, oldProdResp.StatusCode, "old prod code should be rejected by scope resolution: %s", ReadResponseBody(t, oldProdResp))
	oldProdErr := AssertError(t, oldProdResp, "FORBIDDEN")
	require.Equal(t, "invalid workspace", oldProdErr.Error.Message)

	oldTestResp := client.Get(managementEnvironmentWorkspacePath("test", TenantCode, code))
	defer oldTestResp.Body.Close()
	require.Equal(t, http.StatusForbidden, oldTestResp.StatusCode, "old test code should be rejected by scope resolution: %s", ReadResponseBody(t, oldTestResp))
	oldTestErr := AssertError(t, oldTestResp, "FORBIDDEN")
	require.Equal(t, "invalid workspace", oldTestErr.Error.Message)

	testPolicyRecipient := fmt.Sprintf("sandbox-%d@test.example.com", time.Now().UnixNano()%1000000)
	testUpdateResp := client.Put(managementEnvironmentWorkspacePath("test", TenantCode, newCode), map[string]any{
		"is_active":                false,
		"default_locale":           "es-CL",
		"test_recipient_mode":      "append",
		"test_recipient_addresses": []string{testPolicyRecipient},
	})
	defer testUpdateResp.Body.Close()
	RequireStatus(t, testUpdateResp, http.StatusOK)

	updatedTest := mustGetWorkspace(t, client, managementEnvironmentWorkspacePath("test", TenantCode, newCode))
	updatedProd := mustGetWorkspace(t, client, managementEnvironmentWorkspacePath("prod", TenantCode, newCode))
	require.False(t, updatedTest.IsActive)
	require.Equal(t, "append", updatedTest.TestRecipientMode)
	require.Equal(t, []string{testPolicyRecipient}, updatedTest.TestRecipientAddresses)
	require.NotNil(t, updatedTest.DefaultLocale)
	require.Equal(t, "es-CL", *updatedTest.DefaultLocale)

	require.True(t, updatedProd.IsActive)
	require.Equal(t, "replace", updatedProd.TestRecipientMode)
	require.Empty(t, updatedProd.TestRecipientAddresses)
	require.Nil(t, updatedProd.DefaultLocale)

	invalidProdPolicyResp := client.Put(managementEnvironmentWorkspacePath("prod", TenantCode, newCode), map[string]any{
		"test_recipient_mode":      "replace",
		"test_recipient_addresses": []string{"should-not-work@test.example.com"},
	})
	defer invalidProdPolicyResp.Body.Close()
	RequireStatus(t, invalidProdPolicyResp, http.StatusUnprocessableEntity)
	AssertError(t, invalidProdPolicyResp, "VALIDATION_ERROR")
}

func TestEnv02_APIKeyPrefixesAndDataPlaneEnvironmentIsolation(t *testing.T) {
	ensureEnvironmentBaseline(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	workspaceCode := fmt.Sprintf("envdata-%d", time.Now().UnixNano()%1000000)
	mustCreateLogicalWorkspacePair(t, client, workspaceCode, "Environment Data Plane")

	templateSlug := fmt.Sprintf("envdata-type-%d", time.Now().UnixNano()%1000000)
	prodPath := managementWorkspacePath(TenantCode, workspaceCode)
	testPath := managementEnvironmentWorkspacePath("test", TenantCode, workspaceCode)
	mustCreateEnvironmentTemplateSetup(t, client, prodPath, templateSlug, "Environment Prod Template", nil)
	mustCreateEnvironmentTemplateSetup(t, client, testPath, templateSlug, "Environment Test Template", nil)
	testSandboxRecipient := fmt.Sprintf("sandbox-dataplane-%d@test.example.com", time.Now().UnixNano()%1000000)
	mustConfigureWorkspaceTestRecipients(t, client, testPath, "replace", []string{testSandboxRecipient})

	prodKey := mustCreateAPIKeyAtWorkspacePath(t, client, prodPath, "prod")
	testKey := mustCreateAPIKeyAtWorkspacePath(t, client, testPath, "test")
	require.True(t, strings.HasPrefix(prodKey, "senda_prod_"), "expected prod API key prefix, got %q", prodKey)
	require.True(t, strings.HasPrefix(testKey, "senda_test_"), "expected test API key prefix, got %q", testKey)

	ref := fmt.Sprintf("%s:%s:%s", TenantCode, workspaceCode, templateSlug)
	prodRecipient := fmt.Sprintf("prod-dataplane-%d@test.example.com", time.Now().UnixNano()%1000000)
	testRecipient := fmt.Sprintf("test-dataplane-%d@test.example.com", time.Now().UnixNano()%1000000)

	prodSend := mustSendInEnvironment(t, prodKey, ref, prodRecipient)
	require.Len(t, prodSend.TrackingIDs, 1)
	require.Equal(t, prodRecipient, prodSend.TrackingIDs[0].To)

	testSend := mustSendInEnvironment(t, testKey, ref, testRecipient)
	require.Len(t, testSend.TrackingIDs, 1)
	require.Equal(t, testSandboxRecipient, testSend.TrackingIDs[0].To)

	mustWaitForEmailStatusInEnvironment(t, client, "prod", TenantCode, workspaceCode, prodSend.TrackingIDs[0].TrackingID, "sent", 45*time.Second)
	mustWaitForEmailStatusInEnvironment(t, client, "test", TenantCode, workspaceCode, testSend.TrackingIDs[0].TrackingID, "sent", 45*time.Second)

	prodDataList := mustListDataPlaneEmails(t, prodKey, "recipient="+prodRecipient)
	require.Len(t, prodDataList.Items, 1)
	require.Equal(t, prodSend.TrackingIDs[0].TrackingID, prodDataList.Items[0].TrackingID)
	require.Equal(t, prodRecipient, prodDataList.Items[0].RecipientEmail)

	prodCannotSeeTest := mustListDataPlaneEmails(t, prodKey, "recipient="+testRecipient)
	require.Empty(t, prodCannotSeeTest.Items)

	testDataList := mustListDataPlaneEmails(t, testKey, "recipient="+testSandboxRecipient)
	require.Len(t, testDataList.Items, 1)
	require.Equal(t, testSend.TrackingIDs[0].TrackingID, testDataList.Items[0].TrackingID)
	require.Equal(t, testSandboxRecipient, testDataList.Items[0].RecipientEmail)

	testCannotSeeProd := mustListDataPlaneEmails(t, testKey, "recipient="+prodRecipient)
	require.Empty(t, testCannotSeeProd.Items)

	prodClient := NewTestClient(t)
	prodClient.SetAPIKey(prodKey)
	crossProdResp := prodClient.Get("/api/v1/emails/" + testSend.TrackingIDs[0].TrackingID)
	defer crossProdResp.Body.Close()
	RequireStatus(t, crossProdResp, http.StatusNotFound)

	testClient := NewTestClient(t)
	testClient.SetAPIKey(testKey)
	crossTestResp := testClient.Get("/api/v1/emails/" + prodSend.TrackingIDs[0].TrackingID)
	defer crossTestResp.Body.Close()
	RequireStatus(t, crossTestResp, http.StatusNotFound)
}

func TestEnv03_ExternalEnvironmentHeaderValidation(t *testing.T) {
	ctx := t.Context()
	_, err := ensureCoreHarness(ctx)
	require.NoError(t, err)

	ensureExternalSetup(t)

	admin := NewTestClient(t)
	admin.LoginAs(SuperadminEmail)
	ensureExternalIntegrationProfile(t, admin)

	prodResp := externalRequest(t, http.MethodGet,
		externalWorkspacePath(WorkspaceCode)+"/template-types",
		nil,
		map[string]string{
			"X-Tenant-Code":       TenantCode,
			"X-Senda-Environment": "prod",
		},
		"",
	)
	defer prodResp.Body.Close()
	RequireStatus(t, prodResp, http.StatusOK)

	testResp := externalRequest(t, http.MethodGet,
		externalWorkspacePath(WorkspaceCode)+"/template-types",
		nil,
		map[string]string{
			"X-Tenant-Code":       TenantCode,
			"X-Senda-Environment": "TEST",
		},
		"",
	)
	defer testResp.Body.Close()
	RequireStatus(t, testResp, http.StatusOK)

	missingHeaderResp := externalRequest(t, http.MethodGet,
		externalWorkspacePath(WorkspaceCode)+"/template-types",
		nil,
		map[string]string{"X-Tenant-Code": TenantCode},
		"",
	)
	defer missingHeaderResp.Body.Close()
	RequireStatus(t, missingHeaderResp, http.StatusBadRequest)
	missingErr := AssertError(t, missingHeaderResp, "BAD_REQUEST")
	require.Contains(t, missingErr.Error.Message, "X-Senda-Environment")

	invalidHeaderResp := externalRequest(t, http.MethodGet,
		externalWorkspacePath(WorkspaceCode)+"/template-types",
		nil,
		map[string]string{
			"X-Tenant-Code":       TenantCode,
			"X-Senda-Environment": "staging",
		},
		"",
	)
	defer invalidHeaderResp.Body.Close()
	RequireStatus(t, invalidHeaderResp, http.StatusBadRequest)
	invalidErr := AssertError(t, invalidHeaderResp, "BAD_REQUEST")
	require.Contains(t, invalidErr.Error.Message, "invalid X-Senda-Environment header")
}

func TestEnv04_RuntimeResetOnlyTestAndDoesNotAffectProd(t *testing.T) {
	ensureEnvironmentBaseline(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	workspaceCode := fmt.Sprintf("envreset-%d", time.Now().UnixNano()%1000000)
	mustCreateLogicalWorkspacePair(t, client, workspaceCode, "Environment Runtime Reset")

	templateSlug := fmt.Sprintf("envreset-type-%d", time.Now().UnixNano()%1000000)
	prodPath := managementWorkspacePath(TenantCode, workspaceCode)
	testPath := managementEnvironmentWorkspacePath("test", TenantCode, workspaceCode)
	mustCreateEnvironmentTemplateSetup(t, client, prodPath, templateSlug, "Runtime Reset Prod Template", nil)
	mustCreateEnvironmentTemplateSetup(t, client, testPath, templateSlug, "Runtime Reset Test Template", nil)
	testSandboxRecipient := fmt.Sprintf("sandbox-runtime-%d@test.example.com", time.Now().UnixNano()%1000000)
	mustConfigureWorkspaceTestRecipients(t, client, testPath, "replace", []string{testSandboxRecipient})

	prodKey := mustCreateAPIKeyAtWorkspacePath(t, client, prodPath, "reset-prod")
	testKey := mustCreateAPIKeyAtWorkspacePath(t, client, testPath, "reset-test")

	ref := fmt.Sprintf("%s:%s:%s", TenantCode, workspaceCode, templateSlug)
	prodRecipient := fmt.Sprintf("runtime-prod-%d@test.example.com", time.Now().UnixNano()%1000000)
	testRecipient := fmt.Sprintf("runtime-test-%d@test.example.com", time.Now().UnixNano()%1000000)

	prodSend := mustSendInEnvironment(t, prodKey, ref, prodRecipient)
	testSend := mustSendInEnvironment(t, testKey, ref, testRecipient)

	mustWaitForEmailStatusInEnvironment(t, client, "prod", TenantCode, workspaceCode, prodSend.TrackingIDs[0].TrackingID, "sent", 45*time.Second)
	mustWaitForEmailStatusInEnvironment(t, client, "test", TenantCode, workspaceCode, testSend.TrackingIDs[0].TrackingID, "sent", 45*time.Second)

	testResetResp := client.Post(testPath+"/runtime/reset", nil)
	defer testResetResp.Body.Close()
	RequireStatus(t, testResetResp, http.StatusNoContent)

	prodGuardResp := client.Post(managementEnvironmentWorkspacePath("prod", TenantCode, workspaceCode)+"/runtime/reset", nil)
	defer prodGuardResp.Body.Close()
	RequireStatus(t, prodGuardResp, http.StatusConflict)
	AssertError(t, prodGuardResp, "TEST_ENVIRONMENT_REQUIRED")

	testDetailResp := client.Get(managementEnvironmentWorkspacePath("test", TenantCode, workspaceCode) + "/emails/" + testSend.TrackingIDs[0].TrackingID)
	defer testDetailResp.Body.Close()
	RequireStatus(t, testDetailResp, http.StatusNotFound)

	prodDetailResp := client.Get(managementEnvironmentWorkspacePath("prod", TenantCode, workspaceCode) + "/emails/" + prodSend.TrackingIDs[0].TrackingID)
	defer prodDetailResp.Body.Close()
	RequireStatus(t, prodDetailResp, http.StatusOK)

	testEmailsAfterReset := mustListEmailsInEnvironment(t, client, "test", TenantCode, workspaceCode)
	require.Empty(t, testEmailsAfterReset.Items)

	prodEmailsAfterReset := mustListEmailsInEnvironment(t, client, "prod", TenantCode, workspaceCode)
	require.NotEmpty(t, prodEmailsAfterReset.Items)
	foundProdTracking := false
	for _, item := range prodEmailsAfterReset.Items {
		if item.TrackingID == prodSend.TrackingIDs[0].TrackingID {
			foundProdTracking = true
			break
		}
	}
	require.True(t, foundProdTracking, "prod email must survive test runtime reset")
}

func TestEnv05_TestRecipientPolicyReplaceAppendAndTemplateTypeOverride(t *testing.T) {
	ensureEnvironmentBaseline(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	workspaceCode := fmt.Sprintf("envpolicy-%d", time.Now().UnixNano()%1000000)
	mustCreateLogicalWorkspacePair(t, client, workspaceCode, "Environment Recipient Policy")

	templateSlug := fmt.Sprintf("envpolicy-type-%d", time.Now().UnixNano()%1000000)
	testPath := managementEnvironmentWorkspacePath("test", TenantCode, workspaceCode)
	setup := mustCreateEnvironmentTemplateSetup(t, client, testPath, templateSlug, "Environment Policy Template", nil)
	apiKey := mustCreateAPIKeyAtWorkspacePath(t, client, testPath, "policy-test")
	ref := fmt.Sprintf("%s:%s:%s", TenantCode, workspaceCode, templateSlug)

	mailpit := NewMailpitClient(t)

	replaceRecipient := fmt.Sprintf("policy-replace-%d@test.example.com", time.Now().UnixNano()%1000000)
	workspaceReplaceResp := client.Put(testPath, map[string]any{
		"test_recipient_mode":      "replace",
		"test_recipient_addresses": []string{replaceRecipient},
	})
	defer workspaceReplaceResp.Body.Close()
	RequireStatus(t, workspaceReplaceResp, http.StatusOK)

	mailpit.ClearMessages()
	replaceSend := mustSendInEnvironment(t, apiKey, ref, fmt.Sprintf("original-replace-%d@test.example.com", time.Now().UnixNano()%1000000))
	require.Len(t, replaceSend.TrackingIDs, 1)
	require.Equal(t, replaceRecipient, replaceSend.TrackingIDs[0].To)
	mustWaitForEmailStatusInEnvironment(t, client, "test", TenantCode, workspaceCode, replaceSend.TrackingIDs[0].TrackingID, "sent", 45*time.Second)
	mailpit.WaitForMessages(1, 30*time.Second)
	mailpit.AssertMessageExists(replaceRecipient)

	appendRecipient := fmt.Sprintf("policy-append-%d@test.example.com", time.Now().UnixNano()%1000000)
	workspaceAppendResp := client.Put(testPath, map[string]any{
		"test_recipient_mode":      "append",
		"test_recipient_addresses": []string{appendRecipient},
	})
	defer workspaceAppendResp.Body.Close()
	RequireStatus(t, workspaceAppendResp, http.StatusOK)

	mailpit.ClearMessages()
	originalAppendRecipient := fmt.Sprintf("original-append-%d@test.example.com", time.Now().UnixNano()%1000000)
	appendSend := mustSendInEnvironment(t, apiKey, ref, originalAppendRecipient)
	require.Len(t, appendSend.TrackingIDs, 2)
	appendTargets := []string{appendSend.TrackingIDs[0].To, appendSend.TrackingIDs[1].To}
	require.ElementsMatch(t, []string{originalAppendRecipient, appendRecipient}, appendTargets)
	for _, item := range appendSend.TrackingIDs {
		mustWaitForEmailStatusInEnvironment(t, client, "test", TenantCode, workspaceCode, item.TrackingID, "sent", 45*time.Second)
	}
	mailpit.WaitForMessages(2, 30*time.Second)
	mailpit.AssertMessageExists(originalAppendRecipient)
	mailpit.AssertMessageExists(appendRecipient)

	templateOverrideRecipient := fmt.Sprintf("policy-template-override-%d@test.example.com", time.Now().UnixNano()%1000000)
	templateTypeUpdateResp := client.Put(testPath+"/template-types/"+templateSlug, map[string]any{
		"test_recipient_mode":      "replace",
		"test_recipient_addresses": []string{templateOverrideRecipient},
	})
	defer templateTypeUpdateResp.Body.Close()
	RequireStatus(t, templateTypeUpdateResp, http.StatusOK)

	templateTypeGetResp := client.Get(testPath + "/template-types/" + templateSlug)
	defer templateTypeGetResp.Body.Close()
	RequireStatus(t, templateTypeGetResp, http.StatusOK)

	var templateTypeBody struct {
		ID                     string   `json:"id"`
		TestRecipientMode      *string  `json:"test_recipient_mode"`
		TestRecipientAddresses []string `json:"test_recipient_addresses"`
	}
	ParseJSONResponse(t, templateTypeGetResp, &templateTypeBody)
	require.Equal(t, setup.TemplateTypeID, templateTypeBody.ID)
	require.NotNil(t, templateTypeBody.TestRecipientMode)
	require.Equal(t, "replace", *templateTypeBody.TestRecipientMode)
	require.Equal(t, []string{templateOverrideRecipient}, templateTypeBody.TestRecipientAddresses)

	mailpit.ClearMessages()
	templateOverrideSend := mustSendInEnvironment(t, apiKey, ref, fmt.Sprintf("original-template-%d@test.example.com", time.Now().UnixNano()%1000000))
	require.Len(t, templateOverrideSend.TrackingIDs, 1)
	require.Equal(t, templateOverrideRecipient, templateOverrideSend.TrackingIDs[0].To)
	mustWaitForEmailStatusInEnvironment(t, client, "test", TenantCode, workspaceCode, templateOverrideSend.TrackingIDs[0].TrackingID, "sent", 45*time.Second)
	mailpit.WaitForMessages(1, 30*time.Second)
	mailpit.AssertMessageExists(templateOverrideRecipient)
}
