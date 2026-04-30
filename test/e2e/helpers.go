//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rendis/senda/internal/adapter/testauth"
	"github.com/stretchr/testify/require"
)

const (
	defaultJWTSecret = "e2e-test-jwt-secret-at-least-32-characters-long"
	defaultIssuer    = "senda-e2e"
	defaultExpiry    = time.Hour
)

// TestClient wraps HTTP client with helpers for Senda API testing.
type TestClient struct {
	baseURL     string
	httpClient  *http.Client
	t           *testing.T
	bearerToken string
	apiKey      string
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
	if email == SuperadminEmail {
		if token := strings.TrimSpace(os.Getenv("SENDA_E2E_SUPERADMIN_TOKEN")); token != "" {
			c.bearerToken = token
			return
		}
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("SENDA_E2E_LOGIN_MODE")), "password_grant") {
		token, err := loginViaPasswordGrant(email)
		require.NoError(c.t, err, "failed to obtain password-grant token for %s", email)
		c.bearerToken = token
		return
	}

	secret := os.Getenv("SENDA_E2E_JWT_SECRET")
	if secret == "" {
		secret = defaultJWTSecret
	}

	token, err := testauth.GenerateToken(email, email, defaultIssuer, secret, defaultExpiry)
	require.NoError(c.t, err, "failed to generate JWT for %s", email)

	c.bearerToken = token
}

func loginViaPasswordGrant(email string) (string, error) {
	tokenURL := strings.TrimSpace(os.Getenv("SENDA_E2E_OIDC_TOKEN_URL"))
	if tokenURL == "" {
		keycloakBaseURL := strings.TrimSpace(os.Getenv("SENDA_E2E_KEYCLOAK_BASE_URL"))
		if keycloakBaseURL == "" {
			keycloakBaseURL = strings.TrimSpace(os.Getenv("KEYCLOAK_BASE_URL"))
		}
		if keycloakBaseURL == "" {
			keycloakBaseURL = "http://localhost:9090"
		}

		realm := strings.TrimSpace(os.Getenv("SENDA_E2E_KEYCLOAK_REALM"))
		if realm == "" {
			realm = "senda"
		}

		tokenURL = strings.TrimRight(keycloakBaseURL, "/") + "/realms/" + realm + "/protocol/openid-connect/token"
	}

	clientID := strings.TrimSpace(os.Getenv("SENDA_E2E_OIDC_CLIENT_ID"))
	if clientID == "" {
		clientID = "senda-e2e-cli"
	}

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("scope", "openid email profile")
	form.Set("client_id", clientID)
	form.Set("username", email)
	form.Set("password", passwordGrantPassword(email))
	if clientSecret := strings.TrimSpace(os.Getenv("SENDA_E2E_OIDC_CLIENT_SECRET")); clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if host := strings.TrimSpace(os.Getenv("SENDA_E2E_OIDC_TOKEN_HOST")); host != "" {
		req.Host = host
		req.Header.Set("Host", host)
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("password grant failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var payload struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	token := strings.TrimSpace(payload.IDToken)
	if token == "" {
		token = strings.TrimSpace(payload.AccessToken)
	}
	if token == "" {
		return "", errors.New("password grant response missing id_token/access_token")
	}

	return token, nil
}

func passwordGrantPassword(email string) string {
	switch email {
	case SuperadminEmail:
		return envOrDefault("SENDA_E2E_SUPERADMIN_PASSWORD", "superadmin")
	case TenantAdminEmail:
		return envOrDefault("SENDA_E2E_TENANT_ADMIN_PASSWORD", "tenant-admin")
	case WorkspaceAdminEmail:
		return envOrDefault("SENDA_E2E_WORKSPACE_ADMIN_PASSWORD", "workspace-admin")
	case WorkspaceEditorEmail:
		return envOrDefault("SENDA_E2E_WORKSPACE_EDITOR_PASSWORD", "workspace-editor")
	case WorkspaceViewerEmail:
		return envOrDefault("SENDA_E2E_WORKSPACE_VIEWER_PASSWORD", "workspace-viewer")
	default:
		return envOrDefault("SENDA_E2E_DEFAULT_LOGIN_PASSWORD", email)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
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
// API keys are sent as "Authorization: Bearer senda_prod_..." or "Authorization: Bearer senda_test_..." (same header, different prefix).
func (c *TestClient) setAuthHeaders(req *http.Request) {
	if c.apiKey != "" {
		// API keys use Bearer token format; the server detects them by the "senda_prod_" / "senda_test_" prefix.
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
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				body := ReadResponseBody(c.t, resp)
				resp.Body.Close()
				c.t.Fatalf("unauthorized waiting for email %s status via %s: HTTP %d: %s", trackingID, path, resp.StatusCode, body)
			}
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
	ID      string              `json:"ID"`
	From    MailpitAddress      `json:"From"`
	To      []MailpitAddress    `json:"To"`
	Subject string              `json:"Subject"`
	Text    string              `json:"Text"`
	HTML    string              `json:"HTML"`
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
	messages, err := m.tryGetMessages()
	require.NoError(m.t, err)
	return messages
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

// TryGetMessageCount returns the total number of messages in Mailpit without failing the test.
func (m *MailpitClient) TryGetMessageCount() (int, error) {
	messages, err := m.tryGetMessages()
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}

func (m *MailpitClient) tryGetMessages() ([]MessageSummary, error) {
	resp, err := m.httpClient.Get(m.baseURL + "/api/v1/messages")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mailpit get messages status=%d", resp.StatusCode)
	}

	var data MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data.Messages, nil
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
		Code      string        `json:"code"`
		Message   string        `json:"message"`
		Details   []interface{} `json:"details"`
		RequestID string        `json:"request_id"`
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
		"SELECT id::text FROM templates WHERE template_type_id = $1::uuid AND deleted_at IS NULL ORDER BY id DESC LIMIT 1",
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

// EnsureDefaultAdapterIdentity seeds a verified default sender identity for an adapter.
func EnsureDefaultAdapterIdentity(t *testing.T, adapterID, email string) {
	t.Helper()
	EnsureAdapterIdentity(t, adapterID, email, true)
}

// EnsureAdapterIdentity seeds a verified email identity for an adapter.
// When isDefault is true, clears other defaults first.
func EnsureAdapterIdentity(t *testing.T, adapterID, email string, isDefault bool) {
	t.Helper()

	conn := dbConn(t)
	ctx := context.Background()

	tx, err := conn.Begin(ctx)
	require.NoError(t, err, "failed to begin adapter identity transaction")
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if isDefault {
		_, err = tx.Exec(ctx,
			`UPDATE adapter_identities
			   SET is_default = false, updated_at = NOW()
			 WHERE adapter_id = $1::uuid
			   AND identity <> $2`,
			adapterID, email,
		)
		require.NoError(t, err, "failed to clear previous default identities")
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO adapter_identities (
		     id, adapter_id, identity, identity_type, status,
		     sending_enabled, is_default, source, last_synced_at, created_at, updated_at
		 )
		 VALUES (
		     gen_random_uuid(), $1::uuid, $2, 'email', 'verified',
		     true, $3, 'provider', NOW(), NOW(), NOW()
		 )
		 ON CONFLICT (adapter_id, identity) DO UPDATE
		     SET status = 'verified',
		         sending_enabled = true,
		         is_default = $3,
		         source = 'provider',
		         last_synced_at = NOW(),
		         updated_at = NOW()`,
		adapterID, email, isDefault,
	)
	require.NoError(t, err, "failed to upsert adapter identity %s", email)

	require.NoError(t, tx.Commit(ctx), "failed to commit adapter identity transaction")
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

func mustWorkspacePath(tenantCode, workspaceCode string) string {
	return fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", tenantCode, workspaceCode)
}

// MustCreateAPIKey creates an API key for the given workspace and fails fast on any error.
func MustCreateAPIKey(t *testing.T, client *TestClient, tenantCode, workspaceCode, suffix string) (string, string) {
	t.Helper()

	req := APIKeyRequest{
		Name: APIKeyNamePrefix + fmt.Sprintf("%s-%d", suffix, time.Now().UnixNano()),
	}
	resp := client.Post(mustWorkspacePath(tenantCode, workspaceCode)+"/api-keys", req)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusCreated)

	var body struct {
		ID    string `json:"id"`
		Key   string `json:"key"`
		Token string `json:"token"`
	}
	ParseJSONResponse(t, resp, &body)
	require.NotEmpty(t, body.ID, "api key id is required")

	key := body.Key
	if key == "" {
		key = body.Token
	}
	require.NotEmpty(t, key, "api key value is required")

	return body.ID, key
}

// MustGetTenantID resolves tenant UUID by code and fails fast if not found.
func MustGetTenantID(t *testing.T, client *TestClient, tenantCode string) string {
	t.Helper()
	resp := client.Get(fmt.Sprintf("/api/v1/manage/tenants/%s", tenantCode))
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &body)
	require.NotEmpty(t, body.ID, "tenant id is required")
	return body.ID
}

// MustGetWorkspaceID resolves workspace UUID by tenant/workspace code and fails fast.
func MustGetWorkspaceID(t *testing.T, client *TestClient, tenantCode, workspaceCode string) string {
	t.Helper()
	resp := client.Get(mustWorkspacePath(tenantCode, workspaceCode))
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, resp, &body)
	require.NotEmpty(t, body.ID, "workspace id is required")
	return body.ID
}

func findMemberIDByEmail(t *testing.T, email string) string {
	t.Helper()
	conn := dbConn(t)
	var memberID string
	err := conn.QueryRow(context.Background(),
		"SELECT id::text FROM members WHERE email = $1 LIMIT 1",
		email,
	).Scan(&memberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	require.NoError(t, err, "failed to lookup member by email")
	return memberID
}

func memberRoleExists(t *testing.T, memberID, role, scopeType, tenantID, workspaceID string) bool {
	t.Helper()
	conn := dbConn(t)
	var marker int

	switch scopeType {
	case "global":
		err := conn.QueryRow(context.Background(),
			`SELECT 1
			   FROM member_roles
			  WHERE member_id = $1::uuid
			    AND role = $2::member_role
			    AND scope_type = 'global'
			  LIMIT 1`,
			memberID, role,
		).Scan(&marker)
		if errors.Is(err, pgx.ErrNoRows) {
			return false
		}
		require.NoError(t, err, "failed to check global member role")
	case "tenant":
		err := conn.QueryRow(context.Background(),
			`SELECT 1
			   FROM member_roles
			  WHERE member_id = $1::uuid
			    AND role = $2::member_role
			    AND scope_type = 'tenant'
			    AND tenant_id = $3::uuid
			  LIMIT 1`,
			memberID, role, tenantID,
		).Scan(&marker)
		if errors.Is(err, pgx.ErrNoRows) {
			return false
		}
		require.NoError(t, err, "failed to check tenant member role")
	case "workspace":
		err := conn.QueryRow(context.Background(),
			`SELECT 1
			   FROM member_roles
			  WHERE member_id = $1::uuid
			    AND role = $2::member_role
			    AND scope_type = 'workspace'
			    AND tenant_id = $3::uuid
			    AND workspace_id = $4::uuid
			  LIMIT 1`,
			memberID, role, tenantID, workspaceID,
		).Scan(&marker)
		if errors.Is(err, pgx.ErrNoRows) {
			return false
		}
		require.NoError(t, err, "failed to check workspace member role")
	default:
		t.Fatalf("unsupported scope type %q", scopeType)
	}

	return true
}

// MustEnsureMemberWithRole ensures a member exists and has the requested role/scope.
func MustEnsureMemberWithRole(t *testing.T, client *TestClient, email, role, scopeType string) {
	t.Helper()

	memberID := findMemberIDByEmail(t, email)
	if memberID == "" {
		resp := client.Post("/api/v1/manage/members", map[string]interface{}{
			"email":        email,
			"display_name": email,
		})
		if resp.StatusCode == http.StatusCreated {
			var body struct {
				ID string `json:"id"`
			}
			ParseJSONResponse(t, resp, &body)
			memberID = body.ID
			require.NotEmpty(t, memberID, "member id is required after creation")
		} else {
			require.Equal(t, http.StatusConflict, resp.StatusCode,
				"expected 201 or 409 creating member, got %d: %s", resp.StatusCode, ReadResponseBody(t, resp))
			memberID = findMemberIDByEmail(t, email)
		}
		resp.Body.Close()
	}
	require.NotEmpty(t, memberID, "member id must be resolved for %s", email)

	tenantID := ""
	workspaceID := ""
	switch scopeType {
	case "tenant":
		tenantID = MustGetTenantID(t, client, TenantCode)
	case "workspace":
		tenantID = MustGetTenantID(t, client, TenantCode)
		workspaceID = MustGetWorkspaceID(t, client, TenantCode, WorkspaceCode)
	case "global":
	default:
		t.Fatalf("unsupported scope type %q", scopeType)
	}

	if memberRoleExists(t, memberID, role, scopeType, tenantID, workspaceID) {
		return
	}

	roleReq := map[string]interface{}{
		"role":       role,
		"scope_type": scopeType,
	}
	if tenantID != "" {
		roleReq["tenant_id"] = tenantID
	}
	if workspaceID != "" {
		roleReq["workspace_id"] = workspaceID
	}

	roleResp := client.Post(fmt.Sprintf("/api/v1/manage/members/%s/roles", memberID), roleReq)
	if roleResp.StatusCode != http.StatusCreated {
		require.Equal(t, http.StatusConflict, roleResp.StatusCode,
			"expected 201 or 409 creating member role, got %d: %s", roleResp.StatusCode, ReadResponseBody(t, roleResp))
	}
	roleResp.Body.Close()

	require.True(t, memberRoleExists(t, memberID, role, scopeType, tenantID, workspaceID),
		"member %s should have role %s@%s", email, role, scopeType)
}

func mustEnsureWorkspaceAdapter(t *testing.T, client *TestClient, tenantCode, workspaceCode string, rateLimitPerSecond int) string {
	t.Helper()

	path := mustWorkspacePath(tenantCode, workspaceCode)
	createResp := client.Post(path+"/adapters", AdapterRequest{
		AdapterType:        AdapterType,
		Name:               AdapterName,
		RateLimitPerSecond: rateLimitPerSecond,
		Config: map[string]interface{}{
			"region":     "us-east-1",
			"access_key": "test",
			"secret_key": "test",
		},
	})
	if createResp.StatusCode != http.StatusCreated {
		require.Equal(t, http.StatusConflict, createResp.StatusCode,
			"expected 201 or 409 creating adapter, got %d: %s", createResp.StatusCode, ReadResponseBody(t, createResp))
	}
	createResp.Body.Close()

	listResp := client.Get(path + "/adapters")
	defer listResp.Body.Close()
	RequireStatus(t, listResp, http.StatusOK)

	var list struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	ParseJSONResponse(t, listResp, &list)

	for _, item := range list.Items {
		if item.Name == AdapterName {
			require.NotEmpty(t, item.ID, "adapter id must not be empty")
			return item.ID
		}
	}

	t.Fatalf("adapter %q not found after setup", AdapterName)
	return ""
}

// MustEnsureTemplateType ensures a template type exists in the target workspace and returns its ID.
func MustEnsureTemplateType(t *testing.T, client *TestClient, tenantCode, workspaceCode, slug, name, description, adapterID string) string {
	t.Helper()

	path := mustWorkspacePath(tenantCode, workspaceCode)
	typePath := fmt.Sprintf("%s/template-types/%s", path, slug)

	getResp := client.Get(typePath)
	if getResp.StatusCode == http.StatusOK {
		var body struct {
			ID        string  `json:"id"`
			AdapterID *string `json:"adapter_id"`
		}
		ParseJSONResponse(t, getResp, &body)
		getResp.Body.Close()
		require.NotEmpty(t, body.ID, "template type id must not be empty")

		if adapterID != "" && (body.AdapterID == nil || *body.AdapterID != adapterID) {
			AssignAdapterToTemplateType(t, body.ID, adapterID)
		}
		return body.ID
	}
	require.Equal(t, http.StatusNotFound, getResp.StatusCode,
		"expected 200 or 404 reading template type, got %d: %s", getResp.StatusCode, ReadResponseBody(t, getResp))
	getResp.Body.Close()

	createResp := client.Post(path+"/template-types", TemplateTypeRequest{
		Slug:           slug,
		Name:           name,
		Description:    description,
		AdapterID:      adapterID,
		VariableSchema: DefaultVariableSchema(),
	})
	if createResp.StatusCode == http.StatusCreated {
		var body struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, createResp, &body)
		createResp.Body.Close()
		require.NotEmpty(t, body.ID, "template type id must not be empty")
		return body.ID
	}
	require.Equal(t, http.StatusConflict, createResp.StatusCode,
		"expected 201 or 409 creating template type, got %d: %s", createResp.StatusCode, ReadResponseBody(t, createResp))
	createResp.Body.Close()

	getResp = client.Get(typePath)
	defer getResp.Body.Close()
	RequireStatus(t, getResp, http.StatusOK)

	var body struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, getResp, &body)
	require.NotEmpty(t, body.ID, "template type id must not be empty")
	return body.ID
}

// MustEnsureTemplate ensures a template exists for the given type and returns template ID.
func MustEnsureTemplate(t *testing.T, client *TestClient, tenantCode, workspaceCode, templateTypeID string) string {
	t.Helper()
	if existing := GetTemplateIDByTypeID(t, templateTypeID); existing != "" {
		return existing
	}

	resp := client.Post(mustWorkspacePath(tenantCode, workspaceCode)+"/templates", map[string]string{
		"template_type_id": templateTypeID,
	})
	if resp.StatusCode != http.StatusCreated {
		require.Equal(t, http.StatusConflict, resp.StatusCode,
			"expected 201 or 409 creating template, got %d: %s", resp.StatusCode, ReadResponseBody(t, resp))
	}
	resp.Body.Close()

	templateID := GetTemplateIDByTypeID(t, templateTypeID)
	require.NotEmpty(t, templateID, "template id must be resolved for type %s", templateTypeID)
	return templateID
}

// MustEnsureVersionPublished ensures at least one version exists and is published for template.
func MustEnsureVersionPublished(t *testing.T, client *TestClient, tenantCode, workspaceCode, templateID string) string {
	t.Helper()

	path := mustWorkspacePath(tenantCode, workspaceCode)
	versionID := GetLatestVersionID(t, templateID)
	if versionID == "" {
		createResp := client.Post(fmt.Sprintf("%s/templates/%s/versions", path, templateID), CreateVersionRequest{
			Subject:       TestSubject,
			PreviewText:   TestPreviewText,
			FromEmail:     TestFromEmail,
			FromName:      TestFromName,
			BodyMJML:      SampleMJML(),
			DefaultLocale: "en",
		})
		defer createResp.Body.Close()
		RequireStatus(t, createResp, http.StatusCreated)

		var body struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, createResp, &body)
		versionID = body.ID
		require.NotEmpty(t, versionID, "version id must not be empty")
	}

	localeResp := client.Put(fmt.Sprintf("%s/templates/%s/versions/%s/locales/en", path, templateID, versionID), CreateLocaleRequest{
		Locale:      "en",
		Subject:     TestSubject,
		PreviewText: TestPreviewText,
		FromEmail:   TestFromEmail,
		FromName:    TestFromName,
		BodyMJML:    SampleMJML(),
	})
	require.Contains(t, []int{http.StatusOK, http.StatusConflict}, localeResp.StatusCode,
		"expected 200 or 409 setting locale, got %d: %s", localeResp.StatusCode, ReadResponseBody(t, localeResp))
	localeResp.Body.Close()

	publishResp := client.Post(fmt.Sprintf("%s/templates/%s/versions/%s/publish", path, templateID, versionID), nil)
	require.Contains(t, []int{http.StatusNoContent, http.StatusConflict}, publishResp.StatusCode,
		"expected 204 or 409 publishing version, got %d: %s", publishResp.StatusCode, ReadResponseBody(t, publishResp))
	publishResp.Body.Close()

	return versionID
}

// EnsureSetup runs onboarding + workspace + adapter + template setup idempotently.
// It is strict by default: unmet preconditions fail tests instead of skipping.
func EnsureSetup(t *testing.T) {
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

	// Baseline workspace and system workspace must exist.
	_ = MustGetWorkspaceID(t, client, TenantCode, WorkspaceCode)
	_ = MustGetWorkspaceID(t, client, TenantCode, SystemWorkspaceCode)

	adapterID := mustEnsureWorkspaceAdapter(t, client, TenantCode, WorkspaceCode, 100)
	EnsureDefaultAdapterIdentity(t, adapterID, TestFromEmail)

	templateTypeID := MustEnsureTemplateType(t, client, TenantCode, WorkspaceCode, TemplateTypeSlug, TemplateTypeName, TemplateTypeDesc, adapterID)
	templateID := MustEnsureTemplate(t, client, TenantCode, WorkspaceCode, templateTypeID)
	_ = MustEnsureVersionPublished(t, client, TenantCode, WorkspaceCode, templateID)

	// Deterministic RBAC actors used by security/error-flow suites.
	MustEnsureMemberWithRole(t, client, WorkspaceEditorEmail, "workspace_editor", "workspace")
	MustEnsureMemberWithRole(t, client, WorkspaceViewerEmail, "workspace_viewer", "workspace")
}

// TemplateTypeInput holds parameters for CreateTemplateType.
type TemplateTypeInput struct {
	Slug   string
	Name   string
	IsBulk bool
}

// CreateTemplateType creates a template type via the manage API.
// Uses the default adapter from EnsureSetup. Accepts 201 or 409 (idempotent).
// Always ensures the adapter default identity is seeded so that sends work.
func (c *TestClient) CreateTemplateType(t *testing.T, in TemplateTypeInput) {
	t.Helper()
	adapterID := mustEnsureWorkspaceAdapter(t, c, TenantCode, WorkspaceCode, 100)
	EnsureDefaultAdapterIdentity(t, adapterID, TestFromEmail)
	body := map[string]any{
		"slug":       in.Slug,
		"name":       in.Name,
		"is_bulk":    in.IsBulk,
		"adapter_id": adapterID,
	}
	path := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s/template-types", TenantCode, WorkspaceCode)
	resp := c.Post(path, body)
	defer resp.Body.Close()
	require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, resp.StatusCode,
		"CreateTemplateType: unexpected status %d: %s", resp.StatusCode, ReadResponseBody(t, resp))
}

// CreateAndPublishTemplate creates a template with a published version containing the given MJML.
// slug is used for looking up the template type, templateSlug is the MJML body label (not a real slug).
// Accepts idempotent outcomes.
func (c *TestClient) CreateAndPublishTemplate(t *testing.T, typeSlug, _ string, bodyMJML string) {
	t.Helper()
	wp := fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces/%s", TenantCode, WorkspaceCode)

	// Resolve template type ID.
	getResp := c.Get(fmt.Sprintf("%s/template-types/%s", wp, typeSlug))
	defer getResp.Body.Close()
	RequireStatus(t, getResp, http.StatusOK)
	var ttBody struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, getResp, &ttBody)
	require.NotEmpty(t, ttBody.ID, "template type ID must not be empty for slug %s", typeSlug)

	// Create template.
	createTplResp := c.Post(wp+"/templates", map[string]string{"template_type_id": ttBody.ID})
	if createTplResp.StatusCode != http.StatusCreated && createTplResp.StatusCode != http.StatusConflict {
		t.Fatalf("CreateAndPublishTemplate: create template: status %d: %s",
			createTplResp.StatusCode, ReadResponseBody(t, createTplResp))
	}
	createTplResp.Body.Close()

	// Resolve template ID.
	templateID := GetTemplateIDByTypeID(t, ttBody.ID)
	require.NotEmpty(t, templateID, "template ID must not be empty for type %s", ttBody.ID)

	// Create version.
	versionID := GetLatestVersionID(t, templateID)
	if versionID == "" {
		createVerResp := c.Post(fmt.Sprintf("%s/templates/%s/versions", wp, templateID), map[string]string{
			"subject":        "E2E Test",
			"preview_text":   "E2E preview",
			"from_email":     TestFromEmail,
			"from_name":      TestFromName,
			"body_mjml":      bodyMJML,
			"default_locale": "en",
		})
		defer createVerResp.Body.Close()
		RequireStatus(t, createVerResp, http.StatusCreated)
		var verBody struct {
			ID string `json:"id"`
		}
		ParseJSONResponse(t, createVerResp, &verBody)
		versionID = verBody.ID
		require.NotEmpty(t, versionID)
	}

	// Set locale.
	localeResp := c.Put(fmt.Sprintf("%s/templates/%s/versions/%s/locales/en", wp, templateID, versionID), map[string]string{
		"subject":      "E2E Test",
		"preview_text": "E2E preview",
		"from_name":    TestFromName,
		"body_mjml":    bodyMJML,
	})
	require.Contains(t, []int{http.StatusOK, http.StatusConflict}, localeResp.StatusCode,
		"set locale: unexpected status %d: %s", localeResp.StatusCode, ReadResponseBody(t, localeResp))
	localeResp.Body.Close()

	// Publish.
	publishResp := c.Post(fmt.Sprintf("%s/templates/%s/versions/%s/publish", wp, templateID, versionID), nil)
	require.Contains(t, []int{http.StatusNoContent, http.StatusConflict}, publishResp.StatusCode,
		"publish: unexpected status %d: %s", publishResp.StatusCode, ReadResponseBody(t, publishResp))
	publishResp.Body.Close()
}

// SendEmailInput holds parameters for SendEmail.
type SendEmailInput struct {
	TemplateTypeSlug string
	TemplateSlug     string // not used in ref, present for future extensibility
	Recipient        string
	Variables        map[string]any
}

// SendEmail sends a single email via POST /api/v1/send using an API key.
// It creates and sets a fresh API key using the superadmin token the client already holds.
func (c *TestClient) SendEmail(t *testing.T, in SendEmailInput) {
	t.Helper()
	_, apiKey := MustCreateAPIKey(t, c, TenantCode, WorkspaceCode, "unsub-send")
	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKey)

	ref := fmt.Sprintf("%s:%s:%s", TenantCode, WorkspaceCode, in.TemplateTypeSlug)
	body := map[string]any{
		"ref": ref,
		"to":  []string{in.Recipient},
	}
	if len(in.Variables) > 0 {
		body["variables"] = in.Variables
	}

	resp := sendClient.Post("/api/v1/send", body)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusAccepted)
}

// RawHTTP performs an HTTP request against the server without authentication.
// Pass nil body for requests with no body (e.g. POST one-click unsubscribe).
func (c *TestClient) RawHTTP(t *testing.T, method, path string, body []byte) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	require.NoError(t, err)
	return resp
}
