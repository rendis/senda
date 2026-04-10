//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	coreHarnessNetworkName = "senda-e2e-net"
	coreHarnessPostgres    = "senda-e2e-postgres"
	coreHarnessMailpit     = "senda-e2e-mailpit"
	coreHarnessApp         = "senda-e2e-senda"

	coreHarnessPostgresDB   = "senda"
	coreHarnessPostgresUser = "senda"
	coreHarnessPostgresPass = "senda"

	coreHarnessMailpitUI   = "8025/tcp"
	coreHarnessMailpitSMTP = "1025/tcp"
	coreHarnessAppPort     = "8080/tcp"
)

type coreHarness struct {
	network  testcontainers.Network
	postgres testcontainers.Container
	mailpit  testcontainers.Container
	app      testcontainers.Container

	baseURL     string
	dbURL       string
	mailpitURL  string
	projectDir  string
	networkName string
}

var (
	coreHarnessOnce sync.Once
	coreHarnessInst *coreHarness
	coreHarnessErr  error
)

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func ensureCoreHarness(ctx context.Context) (*coreHarness, error) {
	coreHarnessOnce.Do(func() {
		coreHarnessInst, coreHarnessErr = startCoreHarness(ctx)
	})
	return coreHarnessInst, coreHarnessErr
}

func startCoreHarness(ctx context.Context) (*coreHarness, error) {
	root := projectRoot()
	h := &coreHarness{
		projectDir:  root,
		networkName: coreHarnessNetworkName,
	}

	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:       coreHarnessNetworkName,
			Driver:     "bridge",
			Attachable: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create test network: %w", err)
	}
	h.network = network

	postgresCtr, dbURL, err := startPostgresContainer(ctx, root, coreHarnessNetworkName)
	if err != nil {
		_ = testcontainers.TerminateContainer(postgresCtr)
		_ = network.Remove(ctx)
		return nil, err
	}
	h.postgres = postgresCtr
	h.dbURL = dbURL

	mailpitCtr, mailpitURL, err := startMailpitContainer(ctx, coreHarnessNetworkName)
	if err != nil {
		_ = testcontainers.TerminateContainer(postgresCtr)
		_ = testcontainers.TerminateContainer(mailpitCtr)
		_ = network.Remove(ctx)
		return nil, err
	}
	h.mailpit = mailpitCtr
	h.mailpitURL = mailpitURL

	appCtr, baseURL, err := startAppContainer(ctx, root, coreHarnessNetworkName)
	if err != nil {
		_ = testcontainers.TerminateContainer(appCtr)
		_ = testcontainers.TerminateContainer(mailpitCtr)
		_ = testcontainers.TerminateContainer(postgresCtr)
		_ = network.Remove(ctx)
		return nil, err
	}
	h.app = appCtr
	h.baseURL = baseURL

	// Make the harness visible to the test helpers.
	_ = os.Setenv("SENDA_BASE_URL", h.baseURL)
	_ = os.Setenv("SENDA_DATABASE_URL", h.dbURL)
	_ = os.Setenv("MAILPIT_URL", h.mailpitURL)
	_ = os.Setenv("SENDA_E2E_JWT_SECRET", defaultJWTSecret)
	_ = os.Setenv("SENDA_E2E_MASTER_KEY", DefaultMasterKey)
	_ = os.Setenv("SENDA_E2E_AWS_REGION", defaultAWSRegion)
	_ = os.Setenv("SENDA_E2E_AWS_ACCESS_KEY_ID", defaultAWSAccessKeyID)
	_ = os.Setenv("SENDA_E2E_AWS_SECRET_ACCESS_KEY", defaultAWSSecretAccessKey)

	return h, nil
}

func startPostgresContainer(ctx context.Context, rootDir, networkName string) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join(rootDir, "docker", "postgres"),
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{"5432/tcp"},
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
		Name: coreHarnessPostgres,
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(2 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start postgres container: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("postgres host: %w", err)
	}
	mappedPort, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("postgres mapped port: %w", err)
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		coreHarnessPostgresUser, coreHarnessPostgresPass, host, mappedPort.Port(), coreHarnessPostgresDB,
	)
	return ctr, dbURL, nil
}

func startMailpitContainer(ctx context.Context, networkName string) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "axllent/mailpit:latest",
		ExposedPorts: []string{"1025/tcp", "8025/tcp"},
		Networks:     []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: []string{"mailpit"},
		},
		Name: coreHarnessMailpit,
		WaitingFor: wait.ForHTTP("/api/v1/messages").
			WithPort(coreHarnessMailpitUI).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(2 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start mailpit container: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("mailpit host: %w", err)
	}
	mappedPort, err := ctr.MappedPort(ctx, "8025/tcp")
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("mailpit mapped port: %w", err)
	}

	return ctr, fmt.Sprintf("http://%s:%s", host, mappedPort.Port()), nil
}

func startAppContainer(ctx context.Context, rootDir, networkName string) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    rootDir,
			Dockerfile: "docker/Dockerfile.e2e",
		},
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"SENDA_E2E_ENABLE_CODE_INJECTORS":       "true",
			"SENDA_E2E_ENABLE_EXTERNAL_INTEGRATION": "true",
			"SENDA_E2E_EXTERNAL_TOKEN":              "external-e2e-token",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: []string{"senda"},
		},
		Name: coreHarnessApp,
		WaitingFor: wait.ForHTTP("/health").
			WithPort(coreHarnessAppPort).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(3 * time.Minute),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start senda container: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("app host: %w", err)
	}
	mappedPort, err := ctr.MappedPort(ctx, "8080/tcp")
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", fmt.Errorf("app mapped port: %w", err)
	}

	return ctr, fmt.Sprintf("http://%s:%s", host, mappedPort.Port()), nil
}

func (h *coreHarness) Close(ctx context.Context) {
	if h == nil {
		return
	}
	if h.app != nil {
		_ = testcontainers.TerminateContainer(h.app)
	}
	if h.mailpit != nil {
		_ = testcontainers.TerminateContainer(h.mailpit)
	}
	if h.postgres != nil {
		_ = testcontainers.TerminateContainer(h.postgres)
	}
	if h.network != nil {
		_ = h.network.Remove(ctx)
	}
}

func TestMain(m *testing.M) {
	if useExternalStackEnv(os.Getenv) {
		code := m.Run()
		os.Exit(code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	h, err := ensureCoreHarness(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting e2e harness: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	h.Close(context.Background())
	os.Exit(code)
}
