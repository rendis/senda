//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
	"github.com/rendis/senda/internal/adapter/crypto"
	sesadapter "github.com/rendis/senda/internal/adapter/ses"
	"github.com/rendis/senda/internal/domain"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/stretchr/testify/require"
)

const (
	defaultMiniStackImage   = "nahuelnucera/ministack:latest"
	sesHarnessPostgresName  = "senda-e2e-ses-postgres"
	sesHarnessBackendName   = "senda-e2e-ministack"
	sesHarnessBridgeName    = "senda-e2e-aws-sim"
	sesHarnessAppName       = "senda-e2e-ses-app"
	sesHarnessPostgresPort  = "5432/tcp"
	sesHarnessBridgePort    = "4566/tcp"
	sesHarnessAppPort       = "8080/tcp"
	sesHarnessNetworkPrefix = "senda-e2e-ses-"
)

type sesLifecycleHarness struct {
	networkName string
	network     testcontainers.Network
	postgres    testcontainers.Container
	backend     testcontainers.Container
	bridge      testcontainers.Container
	app         testcontainers.Container

	baseURL           string
	dbURL             string
	awsSimURL         string
	awsSimInternalURL string
	awsSimControlURL  string
	projectRoot       string
}

type sesSetup struct {
	Client        *TestClient
	AdapterID      string
	ConfigSetName  string
	TopicARN       string
	SubscriptionARN string
}

func TestSESLifecycle01_HappyPath(t *testing.T) {
	h := startSESLifecycleHarness(t)
	h.Activate(t)
	setup := ensureSESTestSetup(t, h)

	verifyProvisioningState(t, setup.Client, setup.AdapterID, setup.ConfigSetName, setup.TopicARN, setup.SubscriptionARN)
	verifyAWSSimResources(t, h, setup.ConfigSetName, setup.TopicARN)

	trackingID := sendThroughSESAdapter(t, h, setup.Client, "happy-path@test.example.com", "ses-happy")
	setup.Client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusSent), 45*time.Second)

	providerMessageID := mustGetProviderMessageIDByTrackingID(t, trackingID)
	require.NotEmpty(t, providerMessageID)
	require.NotEqual(t, smtpProviderMessageID(trackingID), providerMessageID)

	emitSESEvent(t, h, "Delivery", providerMessageID, "happy-path@test.example.com")
	setup.Client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusDelivered), 45*time.Second)
}

func TestSESLifecycle02_BounceCreatesSuppression(t *testing.T) {
	h := startSESLifecycleHarness(t)
	h.Activate(t)
	setup := ensureSESTestSetup(t, h)

	recipient := "bounce-path@test.example.com"
	trackingID := sendThroughSESAdapter(t, h, setup.Client, recipient, "ses-bounce")
	setup.Client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusSent), 45*time.Second)

	providerMessageID := mustGetProviderMessageIDByTrackingID(t, trackingID)
	emitSESEvent(t, h, "Bounce", providerMessageID, recipient)
	setup.Client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusBounced), 45*time.Second)
	assertSuppressionExists(t, "global", recipient)
}

func TestSESLifecycle03_ComplaintCreatesWorkspaceSuppression(t *testing.T) {
	h := startSESLifecycleHarness(t)
	h.Activate(t)
	setup := ensureSESTestSetup(t, h)

	recipient := "complaint-path@test.example.com"
	trackingID := sendThroughSESAdapter(t, h, setup.Client, recipient, "ses-complaint")
	setup.Client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusSent), 45*time.Second)

	providerMessageID := mustGetProviderMessageIDByTrackingID(t, trackingID)
	emitSESEvent(t, h, "Complaint", providerMessageID, recipient)
	setup.Client.WaitForEmailStatus(TenantCode, WorkspaceCode, trackingID, string(domain.StatusComplained), 45*time.Second)
	assertSuppressionExists(t, "workspace", recipient)
}

func TestSESLifecycle04_DeleteAdapterDeprovisionsAWSSim(t *testing.T) {
	h := startSESLifecycleHarness(t)
	h.Activate(t)
	setup := ensureSESTestSetup(t, h)

	resp := setup.Client.Delete(wsPath() + "/adapters/" + setup.AdapterID)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusNoContent)

	waitForAdapterProvisioningRowsDeleted(t, setup.AdapterID, 30*time.Second)
	verifyAWSSimCleanup(t, h, setup.ConfigSetName, setup.TopicARN)
}

func startSESLifecycleHarness(t *testing.T) *sesLifecycleHarness {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	root := projectRootSES()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	networkName := sesHarnessNetworkPrefix + suffix

	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:       networkName,
			Driver:     "bridge",
			Attachable: true,
		},
	})
	require.NoError(t, err)

	h := &sesLifecycleHarness{
		networkName: networkName,
		network:     network,
		projectRoot: root,
	}
	t.Cleanup(func() { h.Close(context.Background()) })

	h.postgres, h.dbURL = startSESPostgresContainer(t, ctx, root, networkName, suffix)
	h.backend = startMiniStackContainer(t, ctx, networkName, suffix)
	h.bridge, h.awsSimURL = startAWSSimBridgeContainer(t, ctx, root, networkName, suffix)
	h.awsSimInternalURL = "http://aws-sim:4566"
	h.awsSimControlURL = h.awsSimURL + "/_aws-sim/control"
	h.app, h.baseURL = startSESAppContainer(t, ctx, root, networkName, suffix)

	return h
}

func (h *sesLifecycleHarness) Activate(t *testing.T) {
	t.Helper()
	t.Setenv("SENDA_BASE_URL", h.baseURL)
	t.Setenv("SENDA_DATABASE_URL", h.dbURL)
	t.Setenv("SENDA_E2E_JWT_SECRET", defaultJWTSecret)
	t.Setenv("SENDA_E2E_MASTER_KEY", DefaultMasterKey)
	t.Setenv("SENDA_AWS_SIM_KIND", "ministack")
	t.Setenv("SENDA_AWS_SIM_ENDPOINT", h.awsSimURL)
	t.Setenv("SENDA_AWS_SIM_CONTROL_ENDPOINT", h.awsSimControlURL)
}

func (h *sesLifecycleHarness) Close(ctx context.Context) {
	if h == nil {
		return
	}
	if h.app != nil {
		_ = testcontainers.TerminateContainer(h.app)
	}
	if h.bridge != nil {
		_ = testcontainers.TerminateContainer(h.bridge)
	}
	if h.backend != nil {
		_ = testcontainers.TerminateContainer(h.backend)
	}
	if h.postgres != nil {
		_ = testcontainers.TerminateContainer(h.postgres)
	}
	if h.network != nil {
		_ = h.network.Remove(ctx)
	}
}

func projectRootSES() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func startSESPostgresContainer(t *testing.T, ctx context.Context, rootDir, networkName, suffix string) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join(rootDir, "docker", "postgres"),
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{sesHarnessPostgresPort},
		Env: map[string]string{
			"POSTGRES_DB":       coreHarnessPostgresDB,
			"POSTGRES_USER":     coreHarnessPostgresUser,
			"POSTGRES_PASSWORD": coreHarnessPostgresPass,
		},
		Cmd: []string{
			"postgres",
			"-c", "shared_preload_libraries=pg_cron",
			"-c", "cron.database_name=" + coreHarnessPostgresDB,
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: []string{"postgres"},
		},
		Tmpfs: map[string]string{
			"/var/lib/postgresql/data": "rw",
		},
		Name: sesHarnessPostgresName + "-" + suffix,
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(2 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, sesHarnessPostgresPort)
	require.NoError(t, err)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		coreHarnessPostgresUser, coreHarnessPostgresPass, host, mappedPort.Port(), coreHarnessPostgresDB,
	)
	return ctr, dbURL
}

func startMiniStackContainer(t *testing.T, ctx context.Context, networkName, suffix string) testcontainers.Container {
	t.Helper()

	image := strings.TrimSpace(os.Getenv("SENDA_AWS_SIM_IMAGE"))
	if image == "" {
		image = defaultMiniStackImage
	}

	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{sesHarnessBridgePort},
		Env: map[string]string{
			"MINISTACK_HOST":        "aws-sim",
			"GATEWAY_PORT":          "4566",
			"AWS_DEFAULT_REGION":    defaultAWSRegion,
			"AWS_ACCESS_KEY_ID":     defaultAWSAccessKeyID,
			"AWS_SECRET_ACCESS_KEY": defaultAWSSecretAccessKey,
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: []string{"ministack"},
		},
		Name: sesHarnessBackendName + "-" + suffix,
		WaitingFor: wait.ForHTTP("/_ministack/health").
			WithPort(sesHarnessBridgePort).
			WithStartupTimeout(3 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	return ctr
}

func startAWSSimBridgeContainer(t *testing.T, ctx context.Context, rootDir, networkName, suffix string) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    rootDir,
			Dockerfile: "docker/Dockerfile.aws-sim-bridge",
		},
		ExposedPorts: []string{sesHarnessBridgePort},
		Env: map[string]string{
			"AWS_SIM_BACKEND_URL":      "http://ministack:4566",
			"AWS_SIM_REGION":           defaultAWSRegion,
			"AWS_SIM_ACCESS_KEY_ID":    defaultAWSAccessKeyID,
			"AWS_SIM_SECRET_ACCESS_KEY": defaultAWSSecretAccessKey,
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: []string{"aws-sim"},
		},
		Name: sesHarnessBridgeName + "-" + suffix,
		WaitingFor: wait.ForHTTP("/_aws-sim/health").
			WithPort(sesHarnessBridgePort).
			WithStartupTimeout(3 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, sesHarnessBridgePort)
	require.NoError(t, err)
	return ctr, fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
}

func startSESAppContainer(t *testing.T, ctx context.Context, rootDir, networkName, suffix string) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    rootDir,
			Dockerfile: "docker/Dockerfile.e2e",
		},
		ExposedPorts: []string{sesHarnessAppPort},
		Env: map[string]string{
			"SENDA_DATABASE_URL":                    "postgres://senda:senda@postgres:5432/senda?sslmode=disable",
			"SENDA_MIGRATIONS_PATH":                 "/migrations",
			"SENDA_OIDC_MODE":                       "test",
			"SENDA_OIDC_TEST_SECRET":                defaultJWTSecret,
			"SENDA_MASTER_KEY":                      DefaultMasterKey,
			"SENDA_SMTP_HOST":                       "",
			"SENDA_TRACKING_BASE_URL":               "http://senda:8080",
			"SENDA_SNS_SKIP_SIGNATURE_VERIFICATION": "true",
			"SENDA_E2E_ENABLE_CODE_INJECTORS":       "true",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: []string{"senda"},
		},
		Name: sesHarnessAppName + "-" + suffix,
		WaitingFor: wait.ForHTTP("/health").
			WithPort(sesHarnessAppPort).
			WithStartupTimeout(3 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := ctr.MappedPort(ctx, sesHarnessAppPort)
	require.NoError(t, err)
	return ctr, fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
}

func ensureSESTestSetup(t *testing.T, h *sesLifecycleHarness) sesSetup {
	t.Helper()

	WaitForServer(t, 30*time.Second)

	client := NewTestClient(t)
	client.LoginAs(SuperadminEmail)

	onboardingResp := client.Post("/api/v1/onboarding/setup", map[string]string{
		"tenant_code": TenantCode,
		"tenant_name": TenantName,
	})
	require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, onboardingResp.StatusCode)
	onboardingResp.Body.Close()

	workspaceResp := client.Post(fmt.Sprintf("/api/v1/manage/tenants/%s/workspaces", TenantCode), map[string]string{
		"code": WorkspaceCode,
		"name": WorkspaceName,
	})
	require.Contains(t, []int{http.StatusCreated, http.StatusConflict}, workspaceResp.StatusCode)
	workspaceResp.Body.Close()

	adapterID := mustCreateSESAdapter(t, client, h.awsSimInternalURL, uniqueName("ses-adapter"))
	prov := autoProvisionTracking(t, client, adapterID)
	waitForProvisioningCompletion(t, client, adapterID, 30*time.Second)

	mustCreateAWSSimEmailIdentity(t, h.awsSimURL, "mail.test.example.com")
	domainIdentityID := syncSESIdentities(t, client, adapterID, "mail.test.example.com")
	require.NotEmpty(t, domainIdentityID)
	emailIdentityID := createManualSESIdentity(t, client, adapterID, TestFromEmail)
	setDefaultSESIdentity(t, client, adapterID, emailIdentityID)

	templateTypeID := MustEnsureTemplateType(t, client, TenantCode, WorkspaceCode, TemplateTypeSlug, TemplateTypeName, TemplateTypeDesc, adapterID)
	templateID := MustEnsureTemplate(t, client, TenantCode, WorkspaceCode, templateTypeID)
	_ = MustEnsureVersionPublished(t, client, TenantCode, WorkspaceCode, templateID)

	return sesSetup{
		Client:         client,
		AdapterID:      adapterID,
		ConfigSetName:  prov.ConfigSetName,
		TopicARN:       prov.TopicARN,
		SubscriptionARN: prov.SubscriptionARN,
	}
}

func mustCreateSESAdapter(t *testing.T, client *TestClient, endpoint, name string) string {
	t.Helper()

	resp := client.Post(wsPath()+"/adapters", AdapterRequest{
		Name:        name,
		AdapterType: AdapterType,
		Config: map[string]interface{}{
			"region":            defaultAWSRegion,
			"access_key_id":     defaultAWSAccessKeyID,
			"secret_access_key": defaultAWSSecretAccessKey,
			"endpoint_url":      endpoint,
		},
		RateLimitPerSecond: 100,
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

type provisionResult struct {
	ConfigSetName   string `json:"configuration_set_name"`
	TopicARN        string `json:"topic_arn"`
	SubscriptionARN string `json:"subscription_arn"`
	WebhookURL      string `json:"webhook_url"`
	Steps           []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"steps"`
}

func autoProvisionTracking(t *testing.T, client *TestClient, adapterID string) provisionResult {
	t.Helper()

	resp := client.Post(wsPath()+"/adapters/"+adapterID+"/auto-provision-tracking", nil)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var result provisionResult
	ParseJSONResponse(t, resp, &result)
	require.NotEmpty(t, result.ConfigSetName)
	require.NotEmpty(t, result.TopicARN)
	require.NotEmpty(t, result.SubscriptionARN)
	return result
}

func waitForProvisioningCompletion(t *testing.T, client *TestClient, adapterID string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := client.Get(wsPath() + "/adapters/" + adapterID + "/provisioning-status")
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			time.Sleep(200 * time.Millisecond)
			continue
		}
		var body struct {
			Status string `json:"status"`
			Steps  []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"steps"`
		}
		ParseJSONResponse(t, resp, &body)
		resp.Body.Close()
		if body.Status == "completed" {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for provisioning completion")
}

func syncSESIdentities(t *testing.T, client *TestClient, adapterID, domain string) string {
	t.Helper()
	resp := client.Post(wsPath()+"/adapters/"+adapterID+"/identities/sync", nil)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var body []struct {
		ID       string `json:"id"`
		Identity string `json:"identity"`
		Status   string `json:"status"`
	}
	ParseJSONResponse(t, resp, &body)
	for _, item := range body {
		if item.Identity == domain {
			require.Equal(t, "verified", item.Status)
			return item.ID
		}
	}
	t.Fatalf("domain identity %q not found after sync", domain)
	return ""
}

func createManualSESIdentity(t *testing.T, client *TestClient, adapterID, email string) string {
	t.Helper()
	resp := client.Post(wsPath()+"/adapters/"+adapterID+"/identities", map[string]any{
		"identity": email,
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

func setDefaultSESIdentity(t *testing.T, client *TestClient, adapterID, identityID string) {
	t.Helper()
	resp := client.Post(wsPath()+"/adapters/"+adapterID+"/identities/"+identityID+"/set-default", nil)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusNoContent)
}

func sendThroughSESAdapter(t *testing.T, h *sesLifecycleHarness, client *TestClient, recipient, suffix string) string {
	t.Helper()
	apiKeyValue := createAPIKey(t, client, suffix)

	sendClient := NewTestClient(t)
	sendClient.SetAPIKey(apiKeyValue)

	resp := sendClient.Post("/api/v1/send", SendRequest{
		Ref: fmt.Sprintf("%s:%s:%s", TenantCode, WorkspaceCode, TemplateTypeSlug),
		To:  []string{recipient},
		Variables: map[string]interface{}{
			"first_name":   "SES",
			"company_name": "MiniStack",
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
	return body.TrackingIDs[0].TrackingID
}


func dumpSESHarnessLogs(t *testing.T, h *sesLifecycleHarness) {
	t.Helper()
	for _, target := range []struct {
		name string
		ctr  testcontainers.Container
	}{
		{name: "app", ctr: h.app},
		{name: "aws-sim", ctr: h.bridge},
		{name: "ministack", ctr: h.backend},
	} {
		if target.ctr == nil {
			continue
		}
		reader, err := target.ctr.Logs(context.Background())
		if err != nil {
			t.Logf("failed to read %s logs: %v", target.name, err)
			continue
		}
		data, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			t.Logf("failed to consume %s logs: %v", target.name, readErr)
			continue
		}
		t.Logf("%s logs:\n%s", target.name, string(data))
	}
}

func mustGetProviderMessageIDByTrackingID(t *testing.T, trackingID string) string {
	t.Helper()
	conn := dbConn(t)

	var providerMessageID string
	err := conn.QueryRow(context.Background(),
		`SELECT COALESCE(provider_message_id, '')
		   FROM emails
		  WHERE tracking_id = $1`,
		trackingID,
	).Scan(&providerMessageID)
	require.NoError(t, err)
	require.NotEmpty(t, providerMessageID)
	return providerMessageID
}

func emitSESEvent(t *testing.T, h *sesLifecycleHarness, notificationType, providerMessageID, recipient string) {
	t.Helper()
	payload := map[string]any{
		"notification_type":  notificationType,
		"provider_message_id": providerMessageID,
		"recipient":          recipient,
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, h.awsSimControlURL+"/ses-events", strings.NewReader(string(body)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)
}

func verifyProvisioningState(t *testing.T, client *TestClient, adapterID, configSetName, topicARN, subscriptionARN string) {
	t.Helper()

	cfg := mustDecryptSESAdapterConfig(t, adapterID)
	require.Equal(t, configSetName, cfg.ConfigurationSetName)

	resp := client.Get(wsPath() + "/adapters/" + adapterID + "/provisioning-status")
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var status struct {
		Status string `json:"status"`
		Steps  []struct {
			Name         string `json:"name"`
			Status       string `json:"status"`
			ResourceARN  string `json:"resource_arn"`
			ResourceName string `json:"resource_name"`
		} `json:"steps"`
	}
	ParseJSONResponse(t, resp, &status)
	require.Equal(t, "completed", status.Status)

	stepIndex := map[string]struct {
		Status       string
		ResourceARN  string
		ResourceName string
	}{}
	for _, step := range status.Steps {
		stepIndex[step.Name] = struct {
			Status       string
			ResourceARN  string
			ResourceName string
		}{Status: step.Status, ResourceARN: step.ResourceARN, ResourceName: step.ResourceName}
	}

	require.Equal(t, "completed", stepIndex[domain.StepCreateConfigurationSet].Status)
	require.Equal(t, configSetName, stepIndex[domain.StepCreateConfigurationSet].ResourceName)
	require.Equal(t, "completed", stepIndex[domain.StepCreateSNSTopic].Status)
	require.Equal(t, topicARN, stepIndex[domain.StepCreateSNSTopic].ResourceARN)
	require.Equal(t, "completed", stepIndex[domain.StepSubscribeWebhook].Status)
	require.Equal(t, subscriptionARN, stepIndex[domain.StepSubscribeWebhook].ResourceARN)
	require.Equal(t, "completed", stepIndex[domain.StepVerifySubscription].Status)
}

func verifyAWSSimResources(t *testing.T, h *sesLifecycleHarness, configSetName, topicARN string) {
	t.Helper()

	sesClient, err := newAWSSimSESV2Client(context.Background(), h.awsSimURL)
	require.NoError(t, err)
	cfgSets, err := sesClient.ListConfigurationSets(context.Background(), &sesv2.ListConfigurationSetsInput{})
	require.NoError(t, err)
	require.Contains(t, cfgSets.ConfigurationSets, configSetName)

	snsClient, err := newAWSSimSNSClient(context.Background(), h.awsSimURL)
	require.NoError(t, err)
	topics, err := snsClient.ListTopics(context.Background(), &sns.ListTopicsInput{})
	require.NoError(t, err)

	foundTopic := false
	for _, topic := range topics.Topics {
		if aws.ToString(topic.TopicArn) == topicARN {
			foundTopic = true
			subs, err := snsClient.ListSubscriptionsByTopic(context.Background(), &sns.ListSubscriptionsByTopicInput{
				TopicArn: aws.String(topicARN),
			})
			require.NoError(t, err)
			require.NotEmpty(t, subs.Subscriptions)
			for _, sub := range subs.Subscriptions {
				require.NotEqual(t, "PendingConfirmation", aws.ToString(sub.SubscriptionArn))
			}
		}
	}
	require.True(t, foundTopic, "topic %s not found in aws-sim", topicARN)

	state := getAWSSimState(t, h)
	require.Len(t, state.EventDestinations, 1)
	require.Equal(t, configSetName, state.EventDestinations[0].ConfigurationSetName)
	require.Equal(t, topicARN, state.EventDestinations[0].TopicARN)
}

func verifyAWSSimCleanup(t *testing.T, h *sesLifecycleHarness, configSetName, topicARN string) {
	t.Helper()

	sesClient, err := newAWSSimSESV2Client(context.Background(), h.awsSimURL)
	require.NoError(t, err)
	cfgSets, err := sesClient.ListConfigurationSets(context.Background(), &sesv2.ListConfigurationSetsInput{})
	require.NoError(t, err)
	require.NotContains(t, cfgSets.ConfigurationSets, configSetName)

	snsClient, err := newAWSSimSNSClient(context.Background(), h.awsSimURL)
	require.NoError(t, err)
	topics, err := snsClient.ListTopics(context.Background(), &sns.ListTopicsInput{})
	require.NoError(t, err)
	for _, topic := range topics.Topics {
		require.NotEqual(t, topicARN, aws.ToString(topic.TopicArn))
	}

	state := getAWSSimState(t, h)
	require.Empty(t, state.EventDestinations)
}

func getAWSSimState(t *testing.T, h *sesLifecycleHarness) awssimState {
	t.Helper()

	resp, err := http.Get(h.awsSimURL + "/_aws-sim/state")
	require.NoError(t, err)
	defer resp.Body.Close()
	RequireStatus(t, resp, http.StatusOK)

	var state awssimState
	ParseJSONResponse(t, resp, &state)
	return state
}

type awssimState struct {
	EventDestinations []struct {
		ConfigurationSetName string `json:"configuration_set_name"`
		EventDestinationName string `json:"event_destination_name"`
		TopicARN             string `json:"topic_arn"`
	} `json:"event_destinations"`
}

func waitForAdapterProvisioningRowsDeleted(t *testing.T, adapterID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn := dbConn(t)
		var count int
		err := conn.QueryRow(context.Background(),
			`SELECT COUNT(1) FROM adapter_provisioning_steps WHERE adapter_id = $1::uuid`,
			adapterID,
		).Scan(&count)
		require.NoError(t, err)
		if count == 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for adapter provisioning rows deletion")
}

func mustDecryptSESAdapterConfig(t *testing.T, adapterID string) sesadapter.Config {
	t.Helper()

	conn := dbConn(t)
	var encrypted []byte
	err := conn.QueryRow(context.Background(),
		"SELECT config_encrypted FROM adapters WHERE id = $1::uuid",
		adapterID,
	).Scan(&encrypted)
	require.NoError(t, err)

	aesCrypto, err := crypto.NewAESCrypto(DefaultMasterKey)
	require.NoError(t, err)

	plaintext, err := aesCrypto.Decrypt(encrypted)
	require.NoError(t, err)

	var cfg sesadapter.Config
	require.NoError(t, json.Unmarshal(plaintext, &cfg))
	return cfg
}

func assertSuppressionExists(t *testing.T, table, email string) {
	t.Helper()

	conn := dbConn(t)
	var count int
	var query string
	switch table {
	case "global":
		query = "SELECT COUNT(1) FROM suppression_global WHERE email = $1 AND removed_at IS NULL"
	case "workspace":
		query = "SELECT COUNT(1) FROM suppression_workspace WHERE email = $1 AND removed_at IS NULL"
	default:
		t.Fatalf("unsupported suppression table %q", table)
	}
	require.NoError(t, conn.QueryRow(context.Background(), query, email).Scan(&count))
	require.Greater(t, count, 0, "expected suppression entry for %s", email)
}

func smtpProviderMessageID(trackingID string) string {
	return fmt.Sprintf("<trk-%s@senda>", trackingID)
}
