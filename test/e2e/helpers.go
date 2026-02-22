//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/senda-app/senda/internal/adapter/testauth"
	"github.com/stretchr/testify/require"
)

const (
	defaultJWTSecret = "e2e-test-jwt-secret-at-least-32-characters-long"
	defaultIssuer    = "senda-e2e"
	defaultExpiry    = time.Hour
)

// TestClient wraps HTTP client with helpers for Senda API testing.
type TestClient struct {
	baseURL    string
	httpClient *http.Client
	t          *testing.T
	bearerToken string
	apiKey     string
}

// NewTestClient creates a test client configured from environment.
func NewTestClient(t *testing.T) *TestClient {
	baseURL := os.Getenv("SENDA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &TestClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		t: t,
	}
}

// SetBearerToken sets the OIDC JWT token for management API calls.
func (c *TestClient) SetBearerToken(token string) {
	c.bearerToken = token
}

// SetAPIKey sets the API Key for data plane calls.
func (c *TestClient) SetAPIKey(key string) {
	c.apiKey = key
}

// LoginAs generates a JWT token for the given email and sets it as bearer token.
func (c *TestClient) LoginAs(email string) {
	secret := os.Getenv("SENDA_E2E_JWT_SECRET")
	if secret == "" {
		secret = defaultJWTSecret
	}

	token, err := testauth.GenerateToken(email, email, defaultIssuer, secret, defaultExpiry)
	require.NoError(c.t, err, "failed to generate JWT for %s", email)

	c.bearerToken = token
}

// Patch makes a PATCH request with JSON body.
func (c *TestClient) Patch(path string, body any) *http.Response {
	bodyBytes, err := json.Marshal(body)
	require.NoError(c.t, err)

	req, err := http.NewRequest("PATCH", c.baseURL+path, bytes.NewReader(bodyBytes))
	require.NoError(c.t, err)

	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Get makes a GET request and returns the response.
func (c *TestClient) Get(path string) *http.Response {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	require.NoError(c.t, err)

	c.setAuthHeaders(req)
	resp, err := c.httpClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Post makes a POST request with JSON body.
func (c *TestClient) Post(path string, body any) *http.Response {
	bodyBytes, err := json.Marshal(body)
	require.NoError(c.t, err)

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(bodyBytes))
	require.NoError(c.t, err)

	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Put makes a PUT request with JSON body.
func (c *TestClient) Put(path string, body any) *http.Response {
	bodyBytes, err := json.Marshal(body)
	require.NoError(c.t, err)

	req, err := http.NewRequest("PUT", c.baseURL+path, bytes.NewReader(bodyBytes))
	require.NoError(c.t, err)

	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// Delete makes a DELETE request.
func (c *TestClient) Delete(path string) *http.Response {
	req, err := http.NewRequest("DELETE", c.baseURL+path, nil)
	require.NoError(c.t, err)

	c.setAuthHeaders(req)
	resp, err := c.httpClient.Do(req)
	require.NoError(c.t, err)

	return resp
}

// setAuthHeaders adds authorization headers to the request.
// API keys are sent as "Authorization: Bearer senda_live_..." (same header, different prefix).
func (c *TestClient) setAuthHeaders(req *http.Request) {
	if c.apiKey != "" {
		// API keys use Bearer token format; the server detects them by the "senda_live_" prefix.
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		return
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
}

// RequireStatus asserts that the response has the expected status code.
func RequireStatus(t *testing.T, resp *http.Response, expectedStatus int) {
	require.Equal(t, expectedStatus, resp.StatusCode,
		"expected status %d, got %d: %s",
		expectedStatus, resp.StatusCode, ReadResponseBody(t, resp))
}

// ReadResponseBody reads and returns the response body (and resets it).
func ReadResponseBody(t *testing.T, resp *http.Response) string {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Reset body for further reading
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return string(body)
}

// ParseJSON unmarshals the response body into v.
func ParseJSON[T any](t *testing.T, resp *http.Response) T {
	var v T
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = json.Unmarshal(body, &v)
	require.NoError(t, err, "failed to parse response: %s", string(body))

	return v
}

// ParseJSONResponse unmarshals a response with error handling.
func ParseJSONResponse[T any](t *testing.T, resp *http.Response, v *T) {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = json.Unmarshal(body, v)
	require.NoError(t, err, "failed to parse response: %s", string(body))
}

// WaitForEmailStatus polls email status until it reaches expected state or timeout.
// Uses workspace-scoped email tracking path.
func (c *TestClient) WaitForEmailStatus(tenantCode, workspaceCode, trackingID, expectedStatus string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	path := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s/emails/%s", tenantCode, workspaceCode, trackingID)

	for {
		select {
		case <-ctx.Done():
			c.t.Fatalf("timeout waiting for email %s to reach status %s", trackingID, expectedStatus)
		case <-ticker.C:
			resp := c.Get(path)
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				continue
			}

			var data struct {
				Status string `json:"status"`
			}
			ParseJSONResponse(c.t, resp, &data)
			resp.Body.Close()

			if data.Status == expectedStatus {
				return
			}
		}
	}
}

// MailpitClient wraps the Mailpit REST API for email verification.
type MailpitClient struct {
	baseURL    string
	httpClient *http.Client
	t          *testing.T
}

// NewMailpitClient creates a Mailpit client.
func NewMailpitClient(t *testing.T) *MailpitClient {
	baseURL := os.Getenv("MAILPIT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8025"
	}

	return &MailpitClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		t: t,
	}
}

// MailpitAddress is the address representation in Mailpit's API.
type MailpitAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

// Message represents an email message from Mailpit (single message endpoint).
type Message struct {
	ID      string             `json:"ID"`
	From    MailpitAddress     `json:"From"`
	To      []MailpitAddress   `json:"To"`
	Subject string             `json:"Subject"`
	Text    string             `json:"Text"`
	HTML    string             `json:"HTML"`
	Headers map[string][]string `json:"Headers"`
}

// MessageSummary is the lightweight response from list endpoint.
type MessageSummary struct {
	ID      string           `json:"ID"`
	From    MailpitAddress   `json:"From"`
	To      []MailpitAddress `json:"To"`
	Subject string           `json:"Subject"`
}

// MessagesResponse is the paginated response from Mailpit.
type MessagesResponse struct {
	Messages []MessageSummary `json:"messages"`
	Total    int              `json:"total"`
	Unread   int              `json:"unread"`
	Count    int              `json:"count"`
}

// GetMessages fetches all messages from Mailpit.
func (m *MailpitClient) GetMessages() []MessageSummary {
	resp, err := m.httpClient.Get(m.baseURL + "/api/v1/messages")
	require.NoError(m.t, err)
	defer resp.Body.Close()

	require.Equal(m.t, http.StatusOK, resp.StatusCode)

	var data MessagesResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(m.t, err)

	return data.Messages
}

// GetMessage fetches a single message by ID.
func (m *MailpitClient) GetMessage(id string) *Message {
	resp, err := m.httpClient.Get(fmt.Sprintf("%s/api/v1/message/%s", m.baseURL, id))
	require.NoError(m.t, err)
	defer resp.Body.Close()

	require.Equal(m.t, http.StatusOK, resp.StatusCode)

	var msg Message
	err = json.NewDecoder(resp.Body).Decode(&msg)
	require.NoError(m.t, err)

	return &msg
}

// SearchMessages searches Mailpit messages by query.
func (m *MailpitClient) SearchMessages(query string) []MessageSummary {
	url := fmt.Sprintf("%s/api/v1/search?query=%s", m.baseURL, query)
	resp, err := m.httpClient.Get(url)
	require.NoError(m.t, err)
	defer resp.Body.Close()

	require.Equal(m.t, http.StatusOK, resp.StatusCode)

	var data MessagesResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(m.t, err)

	return data.Messages
}

// ClearMessages deletes all messages from Mailpit.
func (m *MailpitClient) ClearMessages() {
	req, err := http.NewRequest("DELETE", m.baseURL+"/api/v1/messages", nil)
	require.NoError(m.t, err)

	resp, err := m.httpClient.Do(req)
	require.NoError(m.t, err)
	defer resp.Body.Close()

	require.Equal(m.t, http.StatusOK, resp.StatusCode)
}

// GetMessageCount returns the total number of messages in Mailpit.
func (m *MailpitClient) GetMessageCount() int {
	messages := m.GetMessages()
	return len(messages)
}

// WaitForMessages waits for N messages to arrive in Mailpit or times out.
func (m *MailpitClient) WaitForMessages(expectedCount int, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.t.Fatalf("timeout waiting for %d messages in Mailpit", expectedCount)
		case <-ticker.C:
			if m.GetMessageCount() >= expectedCount {
				return
			}
		}
	}
}

// AssertMessageExists verifies that an email with the given recipient was received.
func (m *MailpitClient) AssertMessageExists(recipient string) *Message {
	messages := m.GetMessages()

	for _, msg := range messages {
		for _, to := range msg.To {
			if to.Address == recipient {
				return m.GetMessage(msg.ID)
			}
		}
	}

	m.t.Fatalf("no message found for recipient %s in Mailpit", recipient)
	return nil
}

// AssertMessageHasSubject verifies that a message with the given subject exists.
func (m *MailpitClient) AssertMessageHasSubject(subject string) *Message {
	messages := m.GetMessages()

	for _, msg := range messages {
		if msg.Subject == subject {
			return m.GetMessage(msg.ID)
		}
	}

	m.t.Fatalf("no message found with subject '%s' in Mailpit", subject)
	return nil
}

// ErrorResponse matches the API error contract.
type ErrorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Details   []interface{} `json:"details"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

// ParseError extracts error details from a response.
func ParseError(t *testing.T, resp *http.Response) ErrorResponse {
	var errResp ErrorResponse
	ParseJSONResponse(t, resp, &errResp)
	return errResp
}

// AssertError checks that a response contains a specific error code.
func AssertError(t *testing.T, resp *http.Response, expectedCode string) ErrorResponse {
	errResp := ParseError(t, resp)
	require.Equal(t, expectedCode, errResp.Error.Code,
		"expected error code %s, got %s: %s",
		expectedCode, errResp.Error.Code, errResp.Error.Message)
	return errResp
}

// dbConn returns a pgx connection to the E2E database.
func dbConn(t *testing.T) *pgx.Conn {
	t.Helper()
	dbURL := os.Getenv("SENDA_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://senda:senda@localhost:5436/senda?sslmode=disable"
	}
	conn, err := pgx.Connect(context.Background(), dbURL)
	require.NoError(t, err, "failed to connect to E2E database")
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

// GetTemplateIDByTypeID finds an existing template ID for a given template type ID.
func GetTemplateIDByTypeID(t *testing.T, templateTypeID string) string {
	t.Helper()
	conn := dbConn(t)
	var id string
	err := conn.QueryRow(context.Background(),
		"SELECT id::text FROM templates WHERE template_type_id = $1::uuid AND deleted_at IS NULL LIMIT 1",
		templateTypeID,
	).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}

// GetLatestVersionID finds the latest version ID for a given template.
func GetLatestVersionID(t *testing.T, templateID string) string {
	t.Helper()
	conn := dbConn(t)
	var id string
	err := conn.QueryRow(context.Background(),
		"SELECT id::text FROM template_versions WHERE template_id = $1::uuid ORDER BY version_number DESC LIMIT 1",
		templateID,
	).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}

// WaitForServer waits for the server to become reachable (up to timeout).
// Use before chaos tests that may have disrupted infrastructure.
func WaitForServer(t *testing.T, timeout time.Duration) {
	t.Helper()
	baseURL := os.Getenv("SENDA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	deadline := time.Now().Add(timeout)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("server did not become reachable within %v", timeout)
}

// AssignAdapterToTemplateType assigns an adapter to a template type directly in the DB.
// This is needed when the template type was created in a prior run without an adapter.
func AssignAdapterToTemplateType(t *testing.T, templateTypeID, adapterID string) {
	t.Helper()
	conn := dbConn(t)
	tag, err := conn.Exec(context.Background(),
		"UPDATE template_types SET adapter_id = $1::uuid, updated_at = NOW() WHERE id = $2::uuid AND deleted_at IS NULL",
		adapterID, templateTypeID,
	)
	require.NoError(t, err, "failed to assign adapter to template type")
	require.True(t, tag.RowsAffected() > 0, "no template type found to update")
}

// WaitForMailpit polls the Mailpit API until it's reachable.
func WaitForMailpit(t *testing.T, timeout time.Duration) {
	t.Helper()
	mailpitURL := os.Getenv("MAILPIT_URL")
	if mailpitURL == "" {
		mailpitURL = "http://localhost:8025"
	}
	deadline := time.Now().Add(timeout)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(mailpitURL + "/api/v1/messages")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("mailpit did not become reachable within %v", timeout)
}

// EnsureSetup runs onboarding + workspace + adapter + template setup idempotently.
// Call this from any test suite that depends on base data (chaos, security, error flows).
// Safe to call multiple times; skips already-created resources.
func EnsureSetup(t *testing.T) {
	t.Helper()
	WaitForServer(t, 30*time.Second)
	WaitForMailpit(t, 30*time.Second)
	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wsPath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	// 1. Onboarding
	resp := client.Post("/api/v1/onboarding/setup", map[string]string{
		"tenant_code": TenantCode,
		"tenant_name": TenantName,
	})
	resp.Body.Close()

	// 2. Workspace
	wsCreatePath := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", TenantCode)
	resp = client.Post(wsCreatePath, map[string]string{
		"code": WorkspaceCode,
		"name": WorkspaceName,
	})
	resp.Body.Close()

	// 3. Adapter
	resp = client.Post(wsPath+"/adapters", AdapterRequest{
		AdapterType:        AdapterType,
		Name:               AdapterName,
		RateLimitPerSecond: 100,
		Config: map[string]interface{}{
			"region":     "us-east-1",
			"access_key": "test",
			"secret_key": "test",
		},
	})
	resp.Body.Close()

	// 4. Template type
	adapterID := ""
	aResp := client.Get(wsPath + "/adapters")
	defer aResp.Body.Close()
	if aResp.StatusCode == http.StatusOK {
		var list struct {
			Items []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		}
		ParseJSONResponse(t, aResp, &list)
		for _, a := range list.Items {
			if a.Name == AdapterName {
				adapterID = a.ID
				break
			}
		}
	}

	resp = client.Post(wsPath+"/template-types", TemplateTypeRequest{
		Slug:           TemplateTypeSlug,
		Name:           TemplateTypeName,
		Description:    TemplateTypeDesc,
		AdapterID:      adapterID,
		VariableSchema: DefaultVariableSchema(),
	})
	resp.Body.Close()

	// 5. Template
	ttResp := client.Get(fmt.Sprintf("%s/template-types/%s", wsPath, TemplateTypeSlug))
	defer ttResp.Body.Close()
	var ttBody struct {
		ID        string  `json:"id"`
		AdapterID *string `json:"adapter_id"`
	}
	if ttResp.StatusCode == http.StatusOK {
		ParseJSONResponse(t, ttResp, &ttBody)
		// Ensure adapter is assigned (may be missing from previous runs).
		if adapterID != "" && (ttBody.AdapterID == nil || *ttBody.AdapterID == "") {
			AssignAdapterToTemplateType(t, ttBody.ID, adapterID)
		}
	}

	if ttBody.ID != "" {
		resp = client.Post(wsPath+"/templates", map[string]string{
			"template_type_id": ttBody.ID,
		})
		resp.Body.Close()

		// 6. Version + publish (only if template created)
		tplID := GetTemplateIDByTypeID(t, ttBody.ID)
		if tplID != "" {
			existingVID := GetLatestVersionID(t, tplID)
			if existingVID == "" {
				vResp := client.Post(fmt.Sprintf("%s/templates/%s/versions", wsPath, tplID), map[string]string{
					"subject":        TestSubject,
					"preview_text":   TestPreviewText,
					"from_email":     TestFromEmail,
					"from_name":      TestFromName,
					"body_mjml":      SampleMJML(),
					"default_locale": "en",
				})
				if vResp.StatusCode == http.StatusCreated {
					var vBody struct{ ID string `json:"id"` }
					ParseJSONResponse(t, vResp, &vBody)
					// Add locale
					client.Put(fmt.Sprintf("%s/templates/%s/versions/%s/locales/en", wsPath, tplID, vBody.ID), CreateLocaleRequest{
						Locale:      "en",
						Subject:     TestSubject,
						PreviewText: TestPreviewText,
						FromEmail:   TestFromEmail,
						FromName:    TestFromName,
						BodyMJML:    SampleMJML(),
					}).Body.Close()
					// Publish
					client.Post(fmt.Sprintf("%s/templates/%s/versions/%s/publish", wsPath, tplID, vBody.ID), nil).Body.Close()
				}
				vResp.Body.Close()
			}
		}
	}
}

