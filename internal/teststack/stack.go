package teststack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	DefaultRealm               = "senda"
	DefaultJWTSecret           = "e2e-test-jwt-secret-at-least-32-characters-long"
	DefaultMasterKey           = "e2e-master-key-must-be-at-least-32-characters"
	DefaultAWSRegion           = "us-east-1"
	DefaultAWSAccessKeyID      = "test-key"
	DefaultAWSSecretAccessKey  = "test-secret"
	defaultMiniStackImage      = "nahuelnucera/ministack:latest"
	defaultSMTPPort            = 1025
	defaultBackendInternalPort = "8080/tcp"
	defaultMailpitUIPort       = "8025/tcp"
	defaultMailpitSMTPPort     = "1025/tcp"
	defaultPostgresPort        = "5432/tcp"
	defaultKeycloakHTTPPort    = "8080/tcp"
	defaultKeycloakHealthPort  = "9000/tcp"
	defaultAWSSimPort          = "4566/tcp"
)

type Mode string

const (
	ModePR      Mode = "pr"
	ModeNightly Mode = "nightly"
)

type Options struct {
	ProjectRoot string
	Mode        Mode
	OutPath     string
}

type Report struct {
	Mode     string        `json:"mode"`
	Services Services      `json:"services"`
	Runtime  RuntimeReport `json:"runtime"`
}

type Services struct {
	Senda      string `json:"senda,omitempty"`
	Mailpit    string `json:"mailpit,omitempty"`
	Keycloak   string `json:"keycloak,omitempty"`
	AWSSim     string `json:"aws_sim,omitempty"`
	Frontend   string `json:"frontend,omitempty"`
}

type RuntimeReport struct {
	Network                     string            `json:"network"`
	Containers                  map[string]string `json:"containers"`
	KeycloakRealm               string            `json:"keycloak_realm"`
	JWTSecret                   string            `json:"jwt_secret"`
	SkipSNSignatureVerification bool              `json:"skip_sns_signature_verification"`
}

type resourceNames struct {
	Network    string
	Postgres   string
	Keycloak   string
	Mailpit    string
	AWSSim     string
	AWSSimBackend string
	App        string
}

func Up(ctx context.Context, opts Options) (*Report, error) {
	if opts.ProjectRoot == "" {
		return nil, fmt.Errorf("project root is required")
	}
	if opts.Mode == "" {
		opts.Mode = ModePR
	}
	if opts.Mode != ModePR && opts.Mode != ModeNightly {
		return nil, fmt.Errorf("unsupported mode %q", opts.Mode)
	}

	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		return nil, fmt.Errorf("disable ryuk: %w", err)
	}

	names := makeResourceNames(opts.Mode)
	cleanupNamedResources(ctx, names)

	if err := os.MkdirAll(filepath.Dir(opts.OutPath), 0o755); err != nil {
		return nil, fmt.Errorf("create report dir: %w", err)
	}
	if err := ensureNamedNetwork(ctx, names.Network); err != nil {
		return nil, fmt.Errorf("create network: %w", err)
	}

	pg, err := startPostgres(ctx, opts.ProjectRoot, names)
	if err != nil {
		return nil, err
	}
	keycloak, err := startKeycloak(ctx, opts.ProjectRoot, names)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, err
	}
	mailpit, err := startMailpit(ctx, names)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, err
	}
	awsSimBackend, err := startMiniStackBackend(ctx, names)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, err
	}
	awsSim, err := startAWSSimBridge(ctx, opts.ProjectRoot, names)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, err
	}

	report := &Report{
		Mode: string(opts.Mode),
		Runtime: RuntimeReport{
			Network: names.Network,
			Containers: map[string]string{
				"postgres":   names.Postgres,
				"keycloak":   names.Keycloak,
				"mailpit":    names.Mailpit,
				"aws_sim":    names.AWSSim,
				"aws_sim_backend": names.AWSSimBackend,
				"senda":      names.App,
			},
			KeycloakRealm:               DefaultRealm,
			JWTSecret:                   DefaultJWTSecret,
			SkipSNSignatureVerification: opts.Mode == ModeNightly,
		},
	}

	report.Services.Keycloak, err = httpURL(ctx, keycloak, defaultKeycloakHTTPPort)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, fmt.Errorf("resolve keycloak endpoint: %w", err)
	}
	report.Services.Mailpit, err = httpURL(ctx, mailpit, defaultMailpitUIPort)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, fmt.Errorf("resolve mailpit endpoint: %w", err)
	}
	report.Services.AWSSim, err = httpURL(ctx, awsSim, defaultAWSSimPort)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, fmt.Errorf("resolve aws-sim endpoint: %w", err)
	}

	app, err := startApp(ctx, opts.ProjectRoot, names, report)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, err
	}
	report.Services.Senda, err = httpURL(ctx, app, defaultBackendInternalPort)
	if err != nil {
		cleanupNamedResources(ctx, names)
		return nil, fmt.Errorf("resolve senda endpoint: %w", err)
	}

	if err := writeReport(opts.OutPath, report); err != nil {
		cleanupNamedResources(ctx, names)
		return nil, err
	}

	_ = awsSimBackend
	_ = pg
	return report, nil
}

func Down(ctx context.Context, outPath string) error {
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		return fmt.Errorf("disable ryuk: %w", err)
	}

	var names []resourceNames
	if report, err := LoadReport(outPath); err == nil && report != nil {
		names = append(names, resourceNames{
			Network:    report.Runtime.Network,
			Postgres:   report.Runtime.Containers["postgres"],
			Keycloak:   report.Runtime.Containers["keycloak"],
			Mailpit:    report.Runtime.Containers["mailpit"],
			AWSSim:     report.Runtime.Containers["aws_sim"],
			AWSSimBackend: report.Runtime.Containers["aws_sim_backend"],
			App:        report.Runtime.Containers["senda"],
		})
	}
	if len(names) == 0 {
		names = append(names, makeResourceNames(ModePR), makeResourceNames(ModeNightly))
	}
	for _, set := range names {
		cleanupNamedResources(ctx, set)
	}
	if outPath != "" {
		_ = os.Remove(outPath)
	}
	return nil
}

func LoadReport(path string) (*Report, error) {
	if path == "" {
		return nil, fmt.Errorf("report path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}
	return &report, nil
}

func makeResourceNames(mode Mode) resourceNames {
	prefix := "senda-stack-" + string(mode)
	return resourceNames{
		Network:    prefix + "-net",
		Postgres:   prefix + "-postgres",
		Keycloak:   prefix + "-keycloak",
		Mailpit:    prefix + "-mailpit",
		AWSSim:     prefix + "-aws-sim",
		AWSSimBackend: prefix + "-aws-sim-backend",
		App:        prefix + "-app",
	}
}

func cleanupNamedResources(ctx context.Context, names resourceNames) {
	for _, name := range []string{names.App, names.AWSSim, names.AWSSimBackend, names.Mailpit, names.Keycloak, names.Postgres} {
		if strings.TrimSpace(name) == "" {
			continue
		}
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
	}
	if strings.TrimSpace(names.Network) != "" {
		_ = exec.CommandContext(ctx, "docker", "network", "rm", names.Network).Run()
	}
}

func ensureNamedNetwork(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("network name is empty")
	}
	cmd := exec.CommandContext(ctx, "docker", "network", "create", "--driver", "bridge", "--attachable", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker network create %s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func startPostgres(ctx context.Context, root string, names resourceNames) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join(root, "docker", "postgres"),
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{defaultPostgresPort},
		Env: map[string]string{
			"POSTGRES_DB":       "senda",
			"POSTGRES_USER":     "senda",
			"POSTGRES_PASSWORD": "senda",
		},
		Tmpfs: map[string]string{
			"/var/lib/postgresql/data": "rw",
		},
		Cmd: []string{
			"postgres",
			"-c", "shared_preload_libraries=pg_cron",
			"-c", "cron.database_name=senda",
		},
		Networks: []string{names.Network},
		NetworkAliases: map[string][]string{
			names.Network: []string{"postgres"},
		},
		Name: names.Postgres,
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(2 * time.Minute),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func startKeycloak(ctx context.Context, root string, names resourceNames) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join(root, "docker", "keycloak"),
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{defaultKeycloakHTTPPort, defaultKeycloakHealthPort},
		Cmd:          []string{"start-dev", "--import-realm"},
		Env: map[string]string{
			"KC_HEALTH_ENABLED":       "true",
			"KEYCLOAK_ADMIN":          "admin",
			"KEYCLOAK_ADMIN_PASSWORD": "admin",
		},
		Networks: []string{names.Network},
		NetworkAliases: map[string][]string{
			names.Network: []string{"keycloak"},
		},
		Name: names.Keycloak,
		WaitingFor: wait.ForHTTP("/health/ready").
			WithPort(nat.Port(defaultKeycloakHealthPort)).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(3 * time.Minute),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func startMailpit(ctx context.Context, names resourceNames) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "axllent/mailpit:latest",
		ExposedPorts: []string{defaultMailpitSMTPPort, defaultMailpitUIPort},
		Env: map[string]string{
			"MP_SMTP_AUTH_ACCEPT_ANY":     "1",
			"MP_SMTP_AUTH_ALLOW_INSECURE": "1",
		},
		Networks: []string{names.Network},
		NetworkAliases: map[string][]string{
			names.Network: []string{"mailpit"},
		},
		Name: names.Mailpit,
		WaitingFor: wait.ForHTTP("/api/v1/messages").
			WithPort(nat.Port(defaultMailpitUIPort)).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(2 * time.Minute),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func startMiniStackBackend(ctx context.Context, names resourceNames) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        resolveAWSSimImage(),
		ExposedPorts: []string{defaultAWSSimPort},
		Env: map[string]string{
			"MINISTACK_HOST":        "aws-sim",
			"GATEWAY_PORT":          "4566",
			"AWS_DEFAULT_REGION":    DefaultAWSRegion,
			"AWS_ACCESS_KEY_ID":     DefaultAWSAccessKeyID,
			"AWS_SECRET_ACCESS_KEY": DefaultAWSSecretAccessKey,
		},
		Networks: []string{names.Network},
		NetworkAliases: map[string][]string{
			names.Network: []string{"ministack"},
		},
		Name: names.AWSSimBackend,
		WaitingFor: wait.ForHTTP("/_ministack/health").
			WithPort(nat.Port(defaultAWSSimPort)).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(3 * time.Minute),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func startAWSSimBridge(ctx context.Context, root string, names resourceNames) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    root,
			Dockerfile: "docker/Dockerfile.aws-sim-bridge",
		},
		ExposedPorts: []string{defaultAWSSimPort},
		Env: map[string]string{
			"AWS_SIM_BACKEND_URL":       "http://ministack:4566",
			"AWS_SIM_REGION":            DefaultAWSRegion,
			"AWS_SIM_ACCESS_KEY_ID":     DefaultAWSAccessKeyID,
			"AWS_SIM_SECRET_ACCESS_KEY": DefaultAWSSecretAccessKey,
		},
		Networks: []string{names.Network},
		NetworkAliases: map[string][]string{
			names.Network: []string{"aws-sim"},
		},
		Name: names.AWSSim,
		WaitingFor: wait.ForHTTP("/_aws-sim/health").
			WithPort(nat.Port(defaultAWSSimPort)).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(3 * time.Minute),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func resolveAWSSimImage() string {
	if image := strings.TrimSpace(os.Getenv("SENDA_AWS_SIM_IMAGE")); image != "" {
		return image
	}
	return defaultMiniStackImage
}

func startApp(ctx context.Context, root string, names resourceNames, report *Report) (testcontainers.Container, error) {
	env := map[string]string{
		"SENDA_DATABASE_URL":                    "postgres://senda:senda@postgres:5432/senda?sslmode=disable",
		"SENDA_MIGRATIONS_PATH":                 "/migrations",
		"SENDA_OIDC_MODE":                       "dual",
		"SENDA_OIDC_DISCOVERY_URL":              "http://keycloak:8080/realms/" + DefaultRealm + "/.well-known/openid-configuration",
		"SENDA_OIDC_CLIENT_ID":                  "senda-web",
		"SENDA_OIDC_TEST_SECRET":                DefaultJWTSecret,
		"SENDA_MASTER_KEY":                      DefaultMasterKey,
		"SENDA_SMTP_HOST":                       "mailpit",
		"SENDA_SMTP_PORT":                       fmt.Sprintf("%d", defaultSMTPPort),
		"SENDA_TRACKING_BASE_URL":               "http://senda:8080",
		"SENDA_SNS_SKIP_SIGNATURE_VERIFICATION": fmt.Sprintf("%t", report.Runtime.SkipSNSignatureVerification),
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    root,
			Dockerfile: "docker/Dockerfile.e2e",
		},
		ExposedPorts: []string{defaultBackendInternalPort},
		Env:          env,
		Networks:     []string{names.Network},
		NetworkAliases: map[string][]string{
			names.Network: []string{"senda"},
		},
		Name: names.App,
		WaitingFor: wait.ForHTTP("/health").
			WithPort(nat.Port(defaultBackendInternalPort)).
			WithStatusCodeMatcher(func(code int) bool { return code == http.StatusOK }).
			WithStartupTimeout(3 * time.Minute),
	}
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

func httpURL(ctx context.Context, ctr testcontainers.Container, port string) (string, error) {
	mapped, err := ctr.MappedPort(ctx, nat.Port(port))
	if err != nil {
		return "", err
	}
	host, err := ctr.Host(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("http://%s:%s", host, mapped.Port()), nil
}

func writeReport(path string, report *Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
