//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rendis/senda/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestInjectorRuntime01_DataPlanePrecedence(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	mailpit := NewMailpitClient(t)

	adapterID := getAdapterID(t, client)
	templateSlug := fmt.Sprintf("injector-runtime-%d", time.Now().UnixNano())
	templateName := "Injector Runtime"
	templateTypeID := MustEnsureTemplateType(t, client, TenantCode, WorkspaceCode, templateSlug, templateName, "Injector runtime coverage", adapterID)
	templateID := MustEnsureTemplate(t, client, TenantCode, WorkspaceCode, templateTypeID)
	publishTemplateVersion(t, client, TenantCode, WorkspaceCode, templateID, templateSlug, injectorRuntimeSubject(), injectorRuntimeMJML())
	resetStudentInjector(t, client)

	apiKeyValue := createAPIKey(t, client, "injector-runtime")

	cases := []struct {
		name      string
		injectors map[string]map[string]interface{}
		expected  string
	}{
		{
			name:      "default falls back to code and locked default",
			injectors: nil,
			expected:  "NAME=Code Student|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status",
		},
		{
			name: "request wins over code on overwriteable field",
			injectors: map[string]map[string]interface{}{
				"student": {"name": "Request Student"},
			},
			expected: "NAME=Request Student|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status",
		},
		{
			name: "locked ignores request and code",
			injectors: map[string]map[string]interface{}{
				"student": {
					"name":   "Req Locked Case",
					"locked": "SHOULD-NOT-WIN",
				},
			},
			expected: "NAME=Req Locked Case|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status",
		},
		{
			name: "partial fallback happens per field",
			injectors: map[string]map[string]interface{}{
				"student": {
					"status": "request-status",
				},
			},
			expected: "NAME=Code Student|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=request-status",
		},
		{
			name: "null explicit resolves empty string",
			injectors: map[string]map[string]interface{}{
				"student": {
					"name": nil,
				},
			},
			expected: "NAME=|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status",
		},
		{
			name: "empty string explicit resolves empty string",
			injectors: map[string]map[string]interface{}{
				"student": {
					"name": "",
				},
			},
			expected: "NAME=|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mailpit.ClearMessages()
			recipient := fmt.Sprintf("injector-%d@test.example.com", time.Now().UnixNano())

			sendClient := NewTestClient(t)
			sendClient.SetAPIKey(apiKeyValue)

			resp := sendClient.Post("/api/v1/send", SendRequest{
				Ref:       fmt.Sprintf("%s:%s:%s", TenantCode, WorkspaceCode, templateSlug),
				To:        []string{recipient},
				Variables: map[string]interface{}{"user_name": "Ana"},
				Injectors: tc.injectors,
			})
			defer resp.Body.Close()
			RequireStatus(t, resp, http.StatusAccepted)

			mailpit.WaitForMessages(1, 20*time.Second)
			msg := mailpit.AssertMessageExists(recipient)
			require.Contains(t, msg.Subject, tc.expected)
			require.Contains(t, msg.HTML, tc.expected)
			require.Contains(t, msg.HTML, "EVENT=Ana")
		})
	}
}

func TestInjectorRuntime02_ManagementTemplateSendAndBulkSend(t *testing.T) {
	EnsureSetup(t)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	mailpit := NewMailpitClient(t)

	adapterID := getAdapterID(t, client)
	templateSlug := fmt.Sprintf("injector-mgmt-%d", time.Now().UnixNano())
	templateTypeID := MustEnsureTemplateType(t, client, TenantCode, WorkspaceCode, templateSlug, "Injector Mgmt", "Injector management flows", adapterID)
	templateID := MustEnsureTemplate(t, client, TenantCode, WorkspaceCode, templateTypeID)
	publishTemplateVersion(t, client, TenantCode, WorkspaceCode, templateID, templateSlug, injectorRuntimeSubject(), injectorRuntimeMJML())
	resetStudentInjector(t, client)

	t.Run("test send uses injectors", func(t *testing.T) {
		mailpit.ClearMessages()
		recipient := fmt.Sprintf("mgmt-test-send-%d@test.example.com", time.Now().UnixNano())

		resp := client.Post(fmt.Sprintf("%s/templates/%s/test-send", wsPath(), templateID), map[string]any{
			"recipient_email": recipient,
			"variables": map[string]any{
				"user_name": "Mgmt",
			},
			"injectors": map[string]any{
				"student": map[string]any{
					"name": "Mgmt Request",
				},
			},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusOK)

		mailpit.WaitForMessages(1, 20*time.Second)
		msg := mailpit.AssertMessageExists(recipient)
		require.Contains(t, msg.Subject, "NAME=Mgmt Request|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status")
		require.Contains(t, msg.HTML, "EVENT=Mgmt")
	})

	t.Run("bulk send propagates injectors per item", func(t *testing.T) {
		mailpit.ClearMessages()

		firstRecipient := fmt.Sprintf("mgmt-bulk-a-%d@test.example.com", time.Now().UnixNano())
		secondRecipient := fmt.Sprintf("mgmt-bulk-b-%d@test.example.com", time.Now().UnixNano())

		resp := client.Post(fmt.Sprintf("%s/templates/%s/bulk-send", wsPath(), templateID), map[string]any{
			"items": []map[string]any{
				{
					"to": firstRecipient,
					"variables": map[string]any{
						"user_name": "BulkOne",
					},
					"injectors": map[string]any{
						"student": map[string]any{
							"name": "Bulk One",
						},
					},
				},
				{
					"to": secondRecipient,
					"variables": map[string]any{
						"user_name": "BulkTwo",
					},
					"injectors": map[string]any{
						"student": map[string]any{
							"status": "bulk-two-status",
						},
					},
				},
			},
		})
		defer resp.Body.Close()
		RequireStatus(t, resp, http.StatusAccepted)

		mailpit.WaitForMessages(2, 30*time.Second)
		first := mailpit.AssertMessageExists(firstRecipient)
		second := mailpit.AssertMessageExists(secondRecipient)

		require.Contains(t, first.Subject, "NAME=Bulk One|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status")
		require.Contains(t, first.HTML, "EVENT=BulkOne")
		require.Contains(t, second.Subject, "NAME=Code Student|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=bulk-two-status")
		require.Contains(t, second.HTML, "EVENT=BulkTwo")
	})
}

func TestInjectorRuntime03_SESMinistackUsesInjectorPrecedence(t *testing.T) {
	h := startSESLifecycleHarness(t)
	h.Activate(t)
	setup := ensureSESTestSetup(t, h)

	client := setup.Client
	resetStudentInjector(t, client)

	templateSlug := fmt.Sprintf("injector-ses-%d", time.Now().UnixNano())
	templateTypeID := MustEnsureTemplateType(t, client, TenantCode, WorkspaceCode, templateSlug, "Injector SES", "SES injector runtime", setup.AdapterID)
	templateID := MustEnsureTemplate(t, client, TenantCode, WorkspaceCode, templateTypeID)
	publishTemplateVersion(t, client, TenantCode, WorkspaceCode, templateID, templateSlug, injectorRuntimeSubject(), injectorRuntimeMJML())

	apiKeyValue := createAPIKey(t, client, "injector-ses")
	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKeyValue)

	recipient := fmt.Sprintf("injector-ses-%d@test.example.com", time.Now().UnixNano())
	resp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: fmt.Sprintf("%s:%s:%s", TenantCode, WorkspaceCode, templateSlug),
		To:  []string{recipient},
		Variables: map[string]interface{}{
			"user_name": "SES",
		},
		Injectors: map[string]map[string]interface{}{
			"student": {
				"name": "SES Request",
			},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		dumpSESHarnessLogs(t, h)
	}
	RequireStatus(t, resp, http.StatusAccepted)

	var body struct {
		TrackingIDs []struct {
			TrackingID string `json:"tracking_id"`
		} `json:"tracking_ids"`
	}
	ParseJSONResponse(t, resp, &body)
	require.Len(t, body.TrackingIDs, 1)
	trackingID := body.TrackingIDs[0].TrackingID

	client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusSent), 45*time.Second)

	subject, _, snapshot := fetchRenderedEmailSnapshot(t, trackingID)
	require.Contains(t, subject, "NAME=SES Request|AGE=22|LOCKED=LOCKED-DEFAULT|STATUS=code-status")
	require.Equal(t, "SES Request", snapshot["student"]["name"])
	require.Equal(t, float64(22), snapshot["student"]["age"])
	require.Equal(t, "LOCKED-DEFAULT", snapshot["student"]["locked"])

	providerMessageID := mustGetProviderMessageIDByTrackingID(t, trackingID)
	emitSESEvent(t, h, "Delivery", providerMessageID, recipient)
	client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusDelivered), 45*time.Second)
}

func resetStudentInjector(t *testing.T, client *TestClient) {
	t.Helper()

	allowOverwrite := true
	locked := false
	fields := []InjectorFieldRequest{
		{
			FieldName:      "name",
			FieldType:      "text",
			Position:       0,
			DefaultValue:   "Default Student",
			AllowOverwrite: &allowOverwrite,
		},
		{
			FieldName:      "age",
			FieldType:      "number",
			Position:       1,
			DefaultValue:   11,
			AllowOverwrite: &allowOverwrite,
		},
		{
			FieldName:      "locked",
			FieldType:      "text",
			Position:       2,
			DefaultValue:   "LOCKED-DEFAULT",
			AllowOverwrite: &locked,
		},
		{
			FieldName:      "status",
			FieldType:      "text",
			Position:       3,
			DefaultValue:   "DEFAULT-STATUS",
			AllowOverwrite: &allowOverwrite,
		},
	}

	resp := client.Post(wsPath()+"/injectors", InjectorRequest{
		Name:        "student",
		Description: "Student runtime injector",
		Fields:      fields,
	})
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		return
	}
	RequireStatus(t, resp, http.StatusConflict)

	for _, field := range fields {
		updateResp := client.Put(
			fmt.Sprintf("%s/injectors/student/fields/%s", wsPath(), field.FieldName),
			map[string]any{
				"default_value":   field.DefaultValue,
				"allow_overwrite": *field.AllowOverwrite,
			},
		)
		RequireStatus(t, updateResp, http.StatusOK)
		updateResp.Body.Close()
	}
}

func publishTemplateVersion(t *testing.T, client *TestClient, tenantCode, workspaceCode, templateID, templateSlug, subject, bodyMJML string) string {
	t.Helper()

	resp := client.Post(fmt.Sprintf("%s/templates/%s/versions", mustWorkspacePath(tenantCode, workspaceCode), templateID), CreateVersionRequest{
		Subject:       subject,
		PreviewText:   "Injector runtime preview",
		FromEmail:     TestFromEmail,
		FromName:      TestFromName,
		BodyMJML:      bodyMJML,
		DefaultLocale: "en",
	})
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var body struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &body)
	require.NotEmpty(t, body.ID)

	publishResp := client.Post(fmt.Sprintf("%s/templates/%s/versions/%s/publish", mustWorkspacePath(tenantCode, workspaceCode), templateID, body.ID), nil)
	defer publishResp.Body.Close()
	require.Contains(t, []int{http.StatusNoContent, http.StatusConflict}, publishResp.StatusCode,
		"expected 204 or 409 publishing %s, got %d: %s", templateSlug, publishResp.StatusCode, ReadResponseBody(t, publishResp))
	return body.ID
}

func fetchRenderedEmailSnapshot(t *testing.T, trackingID string) (string, string, map[string]map[string]any) {
	t.Helper()
	conn := dbConn(t)

	var subjectRendered string
	var bodyMJML string
	var snapshot map[string]map[string]any
	err := conn.QueryRow(context.Background(),
		`SELECT subject_rendered, body_mjml, injectors_snapshot
		   FROM emails
		  WHERE tracking_id = $1`,
		trackingID,
	).Scan(&subjectRendered, &bodyMJML, &snapshot)
	require.NoError(t, err)
	return subjectRendered, bodyMJML, snapshot
}

func injectorRuntimeSubject() string {
	return "NAME={{ injector.student.name }}|AGE={{ injector.student.age }}|LOCKED={{ injector.student.locked }}|STATUS={{ injector.student.status }}"
}

func injectorRuntimeMJML() string {
	return `<mjml><mj-body><mj-section><mj-column><mj-text>NAME={{ injector.student.name }}|AGE={{ injector.student.age }}|LOCKED={{ injector.student.locked }}|STATUS={{ injector.student.status }}|EVENT={{ event.user_name }}</mj-text></mj-column></mj-section></mj-body></mjml>`
}
