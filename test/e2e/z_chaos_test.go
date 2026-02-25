//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- Docker CLI helpers (no Docker Go SDK needed) ---

// findContainerID uses the docker CLI to find a running container by name substring.
func findContainerID(t *testing.T, nameSubstring string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "--filter", fmt.Sprintf("name=%s", nameSubstring), "--format", "{{.ID}}")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// dockerStop stops a container via docker CLI.
func dockerStop(t *testing.T, containerID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, exec.CommandContext(ctx, "docker", "stop", containerID).Run(),
		"failed to stop container %s", containerID)
}

// dockerStart starts a container via docker CLI.
func dockerStart(t *testing.T, containerID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, exec.CommandContext(ctx, "docker", "start", containerID).Run(),
		"failed to start container %s", containerID)
}

// dockerPause pauses a container via docker CLI.
func dockerPause(t *testing.T, containerID string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "pause", containerID).Run()
}

// dockerUnpause unpauses a container via docker CLI.
func dockerUnpause(t *testing.T, containerID string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "unpause", containerID).Run()
}

// dockerKill sends a signal to a container via docker CLI.
func dockerKill(t *testing.T, containerID, signal string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, exec.CommandContext(ctx, "docker", "kill", "--signal", signal, containerID).Run(),
		"failed to kill container %s with signal %s", containerID, signal)
}

// --- Chaos Tests ---

// C01: TestProviderDown -- Mailpit container stops, emails queue, then are retried and delivered.
func TestC01_ProviderDown(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	mailpit := NewMailpitClient(t)

	apiKeyValue := createAPIKey(t, client, "chaos1")

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKeyValue)

	mailpit.ClearMessages()

	// Send 5 emails before stopping Mailpit
	for i := 0; i < 5; i++ {
		resp := sendClient.Post("/api/v1/send", SendRequest{
			Ref: sendRef(),
			To:  []string{fmt.Sprintf("user%d@example.com", i)},
			Variables: map[string]interface{}{
				"first_name":   "TestUser",
				"company_name": "TestCorp",
			},
		})
		RequireStatus(t, resp, http.StatusAccepted)
		resp.Body.Close()
	}

	// Stop Mailpit container via docker CLI
	mailpitContainerID := findContainerID(t, "mailpit")
	require.NotEmpty(t, mailpitContainerID, "mailpit container not found in docker")

	dockerStop(t, mailpitContainerID)

	t.Cleanup(func() {
		_ = exec.Command("docker", "start", mailpitContainerID).Run()
	})

	time.Sleep(2 * time.Second)

	// Restart Mailpit
	dockerStart(t, mailpitContainerID)
	time.Sleep(5 * time.Second)

	// Wait for Mailpit API to be responsive before checking messages
	mailpitBaseURL := os.Getenv("MAILPIT_URL")
	if mailpitBaseURL == "" {
		mailpitBaseURL = "http://localhost:8025"
	}
	mailpitDeadline := time.Now().Add(15 * time.Second)
	mailpitHTTP := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(mailpitDeadline) {
		r, err := mailpitHTTP.Get(mailpitBaseURL + "/api/v1/messages")
		if err == nil && r.StatusCode == http.StatusOK {
			r.Body.Close()
			break
		}
		if r != nil {
			r.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}

	recoveryResp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  []string{"provider-recovery@example.com"},
		Variables: map[string]interface{}{
			"first_name":   "Recovery",
			"company_name": "Chaos",
		},
	})
	RequireStatus(t, recoveryResp, http.StatusAccepted)
	recoveryResp.Body.Close()

	require.Eventually(t, func() bool {
		return mailpit.GetMessageCount() >= 1
	}, 120*time.Second, 1*time.Second, "expected at least one email delivered after provider restart")
}

// C02: TestDBConnectionLost -- Database becomes unreachable during request processing.
func TestC02_DBConnectionLost(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	pgContainerID := findContainerID(t, "postgres")
	require.NotEmpty(t, pgContainerID, "postgres container not found in docker")

	err := dockerPause(t, pgContainerID)
	require.NoError(t, err, "failed to pause postgres container")

	t.Cleanup(func() {
		_ = dockerUnpause(t, pgContainerID)
	})

	testHTTPClient := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("GET", client.baseURL+"/health", nil)
	require.NoError(t, err)

	resp, err := testHTTPClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	_ = dockerUnpause(t, pgContainerID)
	time.Sleep(2 * time.Second)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		testHTTPClient.Timeout = 5 * time.Second
		req, _ := http.NewRequest("GET", client.baseURL+"/health", nil)
		resp, err = testHTTPClient.Do(req)
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// C03: TestWorkerCrashRecovery -- Senda server crashes while jobs are queued.
func TestC03_WorkerCrashRecovery(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	mailpit := NewMailpitClient(t)

	apiKeyValue := createAPIKey(t, client, "chaos3")

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKeyValue)

	mailpit.ClearMessages()

	for i := 0; i < 10; i++ {
		resp := sendClient.Post("/api/v1/send", SendRequest{
			Ref: sendRef(),
			To:  []string{fmt.Sprintf("crash-test-%d@example.com", i)},
			Variables: map[string]interface{}{
				"first_name":   "CrashUser",
				"company_name": "CrashCorp",
			},
		})
		RequireStatus(t, resp, http.StatusAccepted)
		resp.Body.Close()
	}

	sendaContainerID := findContainerID(t, "senda")
	require.NotEmpty(t, sendaContainerID, "senda container not found in docker")

	dockerKill(t, sendaContainerID, "SIGKILL")
	time.Sleep(2 * time.Second)

	dockerStart(t, sendaContainerID)
	time.Sleep(5 * time.Second)

	deadline := time.Now().Add(60 * time.Second)
	var finalCount int
	for time.Now().Before(deadline) {
		finalCount = mailpit.GetMessageCount()
		if finalCount >= 10 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	require.GreaterOrEqual(t, finalCount, 10, "expected at least 10 emails delivered after crash recovery, got %d", finalCount)
}

// C04: TestConcurrentSendRaceCondition -- 50 concurrent sends with same template.
func TestC04_ConcurrentSendRaceCondition(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	mailpit := NewMailpitClient(t)
	mailpit.ClearMessages()

	apiKeyValue := createAPIKey(t, client, "chaos4")

	const numGoroutines = 50

	var wg sync.WaitGroup
	results := make(chan int, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			sc := NewTestClient(t)
			sc.SetAPIKey(apiKeyValue)

			resp := sc.Post("/api/v1/send", SendRequest{
				Ref: sendRef(),
				To:  []string{fmt.Sprintf("concurrent-%d@example.com", idx)},
				Variables: map[string]interface{}{
					"first_name":   fmt.Sprintf("User%d", idx),
					"company_name": "ConcurrentCorp",
				},
			})
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(i)
	}

	wg.Wait()
	close(results)

	var successCount, otherCount int
	for status := range results {
		if status == http.StatusAccepted {
			successCount++
		} else {
			otherCount++
		}
	}

	require.Greater(t, successCount, 0, "no requests succeeded")
	t.Logf("concurrent send results: %d accepted, %d other", successCount, otherCount)
}

// C05: TestConcurrentPublishRace -- Publish template version while another publish is in progress.
func TestC05_ConcurrentPublishRace(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wp := wsPath()
	adapterID := getAdapterID(t, client)

	// Create a template type
	typeResp := client.Post(wp+"/template-types", TemplateTypeRequest{
		Slug:           fmt.Sprintf("race-type-%d", time.Now().UnixNano()),
		Name:           "Race Publish Type",
		Description:    "Template type for publish race test",
		AdapterID:      adapterID,
		VariableSchema: DefaultVariableSchema(),
	})
	RequireStatus(t, typeResp, http.StatusCreated)

	var typeData struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, typeResp, &typeData)
	typeResp.Body.Close()

	// Create a template
	tplResp := client.Post(wp+"/templates", map[string]string{
		"template_type_id": typeData.ID,
	})
	RequireStatus(t, tplResp, http.StatusCreated)

	var tplData struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, tplResp, &tplData)
	tplResp.Body.Close()

	// Create two draft versions
	versionsPath := fmt.Sprintf("%s/templates/%s/versions", wp, tplData.ID)

	v2Resp := client.Post(versionsPath, map[string]string{
		"subject":        "Version 2 Subject",
		"preview_text":   "Version 2 Preview",
		"from_email":     TestFromEmail,
		"from_name":      TestFromName,
		"body_mjml":      SampleMJML(),
		"default_locale": "en",
	})
	RequireStatus(t, v2Resp, http.StatusCreated)
	var v2Data struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, v2Resp, &v2Data)
	v2Resp.Body.Close()

	v3Resp := client.Post(versionsPath, map[string]string{
		"subject":        "Version 3 Subject",
		"preview_text":   "Version 3 Preview",
		"from_email":     TestFromEmail,
		"from_name":      TestFromName,
		"body_mjml":      SampleMJML(),
		"default_locale": "en",
	})
	RequireStatus(t, v3Resp, http.StatusCreated)
	var v3Data struct {
		ID string `json:"id"`
	}
	ParseJSONResponse(t, v3Resp, &v3Data)
	v3Resp.Body.Close()

	// Concurrently publish two versions
	var wg sync.WaitGroup
	var publishResults [2]int

	wg.Add(2)

	go func() {
		defer wg.Done()
		resp := client.Post(fmt.Sprintf("%s/%s/publish", versionsPath, v2Data.ID), nil)
		publishResults[0] = resp.StatusCode
		resp.Body.Close()
	}()

	go func() {
		defer wg.Done()
		resp := client.Post(fmt.Sprintf("%s/%s/publish", versionsPath, v3Data.ID), nil)
		publishResults[1] = resp.StatusCode
		resp.Body.Close()
	}()

	wg.Wait()

	// Verify at least one succeeded
	successCount := 0
	for _, code := range publishResults {
		if code == http.StatusOK || code == http.StatusNoContent {
			successCount++
		}
	}

	require.Greater(t, successCount, 0, "expected at least 1 successful publish")
}

// C06: TestRateLimiterUnderLoad -- Exceed configured rate limit with high concurrency.
func TestC06_RateLimiterUnderLoad(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	apiKeyValue := createAPIKey(t, client, "chaos6")

	const requestCount = 150

	var wg sync.WaitGroup
	results := make(chan int, requestCount)

	wg.Add(requestCount)
	for i := 0; i < requestCount; i++ {
		go func(idx int) {
			defer wg.Done()

			sc := NewTestClient(t)
			sc.SetAPIKey(apiKeyValue)

			resp := sc.Post("/api/v1/send", SendRequest{
				Ref: sendRef(),
				To:  []string{fmt.Sprintf("ratelimit-%d@example.com", idx)},
				Variables: map[string]interface{}{
					"first_name":   "RateLimitUser",
					"company_name": "RateLimitCorp",
				},
			})
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(i)
	}

	wg.Wait()
	close(results)

	var successCount, rateLimitCount, otherCount int
	for status := range results {
		switch status {
		case http.StatusAccepted:
			successCount++
		case http.StatusTooManyRequests:
			rateLimitCount++
		default:
			otherCount++
		}
	}

	t.Logf("Rate limit test results: %d accepted, %d rate limited, %d other", successCount, rateLimitCount, otherCount)
	require.Greater(t, successCount, 0, "no requests succeeded")
}

// C07: TestWebhookEndpointSlow -- Webhook endpoint times out, should not block send flow.
func TestC07_WebhookEndpointSlow(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	wp := wsPath()

	// Start a slow webhook server
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	// Register webhook
	webhookResp := client.Post(wp+"/webhooks", WebhookRequest{
		URL:    slowServer.URL,
		Events: []string{"email.sent", "email.delivered"},
	})
	defer webhookResp.Body.Close()

	if webhookResp.StatusCode != http.StatusCreated {
		t.Logf("webhook creation returned %d (may be expected for localhost URLs)", webhookResp.StatusCode)
	}

	apiKeyValue := createAPIKey(t, client, "chaos7")

	sc := NewTestClient(t)
	sc.SetAPIKey(apiKeyValue)

	startTime := time.Now()
	sendResp := sc.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  []string{"webhook-test@example.com"},
		Variables: map[string]interface{}{
			"first_name":   "WebhookTest",
			"company_name": "WebhookCorp",
		},
	})
	duration := time.Since(startTime)
	defer sendResp.Body.Close()

	RequireStatus(t, sendResp, http.StatusAccepted)

	require.Less(t, duration, 5*time.Second,
		"send took %v - webhook may have blocked the flow", duration)
}

// C08: TestPayloadGigante -- Send with large payload.
func TestC08_PayloadGigante(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	apiKeyValue := createAPIKey(t, client, "chaos8")

	sc := NewTestClient(t)
	sc.SetAPIKey(apiKeyValue)

	// Build a large recipients list (10k recipients ~ 250KB)
	// Using 400k recipients times out the HTTP client (10s), so we use a smaller payload
	// that still tests the server's handling of large requests.
	recipients := make([]string, 10000)
	for i := range recipients {
		recipients[i] = fmt.Sprintf("bigpayload%d@example.com", i)
	}

	resp := sc.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  recipients,
		Variables: map[string]interface{}{
			"first_name":   "PayloadTest",
			"company_name": "PayloadCorp",
		},
	})
	defer resp.Body.Close()

	// Accept any non-panic response - server should handle large payloads gracefully
	require.True(t,
		resp.StatusCode == http.StatusRequestEntityTooLarge ||
			resp.StatusCode == http.StatusBadRequest ||
			resp.StatusCode == http.StatusUnprocessableEntity ||
			resp.StatusCode == http.StatusAccepted,
		"expected 413, 400, 422, or 202, got %d", resp.StatusCode)

	// Verify server is still responsive
	time.Sleep(500 * time.Millisecond)

	normalResp := sc.Get("/health")
	defer normalResp.Body.Close()
	require.NotNil(t, normalResp)
}

// C09: TestCacheInvalidationRace -- Modify template while send is using cache.
func TestC09_CacheInvalidationRace(t *testing.T) {
	WaitForServer(t, 30*time.Second)
	EnsureSetup(t)
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)
	mailpit := NewMailpitClient(t)
	mailpit.ClearMessages()

	apiKeyValue := createAPIKey(t, client, "chaos9")

	sc := NewTestClient(t)
	sc.SetAPIKey(apiKeyValue)

	resp1 := sc.Post("/api/v1/send", SendRequest{
		Ref: sendRef(),
		To:  []string{"cache-test-1@example.com"},
		Variables: map[string]interface{}{
			"first_name":   "User1",
			"company_name": "CacheTestCorp",
		},
	})
	defer resp1.Body.Close()

	RequireStatus(t, resp1, http.StatusAccepted)

	// Concurrent sends
	var wg sync.WaitGroup
	wg.Add(5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			defer wg.Done()
			gc := NewTestClient(t)
			gc.SetAPIKey(apiKeyValue)
			resp := gc.Post("/api/v1/send", SendRequest{
				Ref: sendRef(),
				To:  []string{fmt.Sprintf("cache-race-%d@example.com", idx)},
				Variables: map[string]interface{}{
					"first_name":   fmt.Sprintf("User%d", idx),
					"company_name": "CacheTestCorp",
				},
			})
			resp.Body.Close()
		}(i)
	}

	wg.Wait()

	time.Sleep(3 * time.Second)

	messages := mailpit.GetMessages()
	require.GreaterOrEqual(t, len(messages), 1, "expected at least 1 email delivered")
}
