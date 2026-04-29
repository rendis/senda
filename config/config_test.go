package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_FullYAML(t *testing.T) {
	yaml := `
server:
  host: "127.0.0.1"
  port: 9090
  read_timeout: "10s"
  write_timeout: "20s"
  shutdown_timeout: "5s"
send:
  batch_max_items: 250
database:
  url: "postgres://user:pass@localhost:5432/senda"
  max_open_conns: 50
  min_conns: 20
  conn_max_lifetime: "10m"
  migrate_on_start: false
oidc:
  discovery_url: "https://auth.example.com/.well-known/openid-configuration"
  client_id: "my-client"
  client_secret: "my-secret"
sns:
  expected_topic_arn: "arn:aws:sns:us-east-1:123456789012:SES-Events"
  expected_account_id: "123456789012"
media:
  thumbnail_allowed_hosts:
    - "img.youtube.com"
    - "i.ytimg.com"
  thumbnail_cache_ttl: "24h"
  thumbnail_cache_max_entries: 500
  thumbnail_fetch_timeout: "10s"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
log:
  level: "debug"
  format: "text"
`
	path := writeYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Server
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Server.ReadTimeout != 10*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want %v", cfg.Server.ReadTimeout, 10*time.Second)
	}
	if cfg.Server.WriteTimeout != 20*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want %v", cfg.Server.WriteTimeout, 20*time.Second)
	}
	if cfg.Server.ShutdownTimeout != 5*time.Second {
		t.Errorf("Server.ShutdownTimeout = %v, want %v", cfg.Server.ShutdownTimeout, 5*time.Second)
	}
	if cfg.Send.BatchMaxItems != 250 {
		t.Errorf("Send.BatchMaxItems = %d, want %d", cfg.Send.BatchMaxItems, 250)
	}

	// Database
	if cfg.Database.URL != "postgres://user:pass@localhost:5432/senda" {
		t.Errorf("Database.URL = %q, want postgres URL", cfg.Database.URL)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Errorf("Database.MaxOpenConns = %d, want 50", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MinConns != 20 {
		t.Errorf("Database.MinConns = %d, want 20", cfg.Database.MinConns)
	}
	if cfg.Database.ConnMaxLifetime != "10m" {
		t.Errorf("Database.ConnMaxLifetime = %q, want %q", cfg.Database.ConnMaxLifetime, "10m")
	}
	if cfg.Database.MigrateOnStart != false {
		t.Errorf("Database.MigrateOnStart = %v, want false", cfg.Database.MigrateOnStart)
	}

	// OIDC
	if cfg.OIDC.DiscoveryURL != "https://auth.example.com/.well-known/openid-configuration" {
		t.Errorf("OIDC.DiscoveryURL = %q", cfg.OIDC.DiscoveryURL)
	}
	if cfg.OIDC.ClientID != "my-client" {
		t.Errorf("OIDC.ClientID = %q", cfg.OIDC.ClientID)
	}
	if cfg.OIDC.ClientSecret != "my-secret" {
		t.Errorf("OIDC.ClientSecret = %q", cfg.OIDC.ClientSecret)
	}
	if cfg.SNS.ExpectedTopicArn != "arn:aws:sns:us-east-1:123456789012:SES-Events" {
		t.Errorf("SNS.ExpectedTopicArn = %q", cfg.SNS.ExpectedTopicArn)
	}
	if cfg.SNS.ExpectedAccountID != "123456789012" {
		t.Errorf("SNS.ExpectedAccountID = %q", cfg.SNS.ExpectedAccountID)
	}
	if len(cfg.Media.ThumbnailAllowedHosts) != 2 {
		t.Fatalf("Media.ThumbnailAllowedHosts length = %d, want 2", len(cfg.Media.ThumbnailAllowedHosts))
	}
	if cfg.Media.ThumbnailAllowedHosts[0] != "img.youtube.com" || cfg.Media.ThumbnailAllowedHosts[1] != "i.ytimg.com" {
		t.Errorf("Media.ThumbnailAllowedHosts = %#v", cfg.Media.ThumbnailAllowedHosts)
	}
	if cfg.Media.ThumbnailCacheTTL != 24*time.Hour {
		t.Errorf("Media.ThumbnailCacheTTL = %v, want 24h", cfg.Media.ThumbnailCacheTTL)
	}
	if cfg.Media.ThumbnailCacheMaxEntries != 500 {
		t.Errorf("Media.ThumbnailCacheMaxEntries = %d, want 500", cfg.Media.ThumbnailCacheMaxEntries)
	}
	if cfg.Media.ThumbnailFetchTimeout != 10*time.Second {
		t.Errorf("Media.ThumbnailFetchTimeout = %v, want 10s", cfg.Media.ThumbnailFetchTimeout)
	}

	// Crypto
	if cfg.Crypto.MasterKey != "this-is-a-32-char-master-key!!!!" {
		t.Errorf("Crypto.MasterKey = %q", cfg.Crypto.MasterKey)
	}

	// Log
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.Format != "text" {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, "text")
	}
}

func TestLoad_Defaults(t *testing.T) {
	yaml := `
database:
  url: "postgres://user:pass@localhost:5432/senda"
oidc:
  discovery_url: "https://auth.example.com/.well-known/openid-configuration"
  client_id: "my-client"
  client_secret: "my-secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("default Server.Host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("default Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("default Server.ReadTimeout = %v, want %v", cfg.Server.ReadTimeout, 30*time.Second)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("default Server.WriteTimeout = %v, want %v", cfg.Server.WriteTimeout, 30*time.Second)
	}
	if cfg.Server.ShutdownTimeout != 15*time.Second {
		t.Errorf("default Server.ShutdownTimeout = %v, want %v", cfg.Server.ShutdownTimeout, 15*time.Second)
	}
	if cfg.Send.BatchMaxItems != 100 {
		t.Errorf("default Send.BatchMaxItems = %d, want %d", cfg.Send.BatchMaxItems, 100)
	}
	if cfg.Database.MaxOpenConns != 60 {
		t.Errorf("default Database.MaxOpenConns = %d, want 60", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MinConns != 10 {
		t.Errorf("default Database.MinConns = %d, want 10", cfg.Database.MinConns)
	}
	if cfg.Database.ConnMaxLifetime != "5m" {
		t.Errorf("default Database.ConnMaxLifetime = %q, want %q", cfg.Database.ConnMaxLifetime, "5m")
	}
	if cfg.Database.MigrateOnStart != true {
		t.Errorf("default Database.MigrateOnStart = %v, want true", cfg.Database.MigrateOnStart)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("default Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "json" {
		t.Errorf("default Log.Format = %q, want %q", cfg.Log.Format, "json")
	}
	if cfg.SNS.SkipSignatureVerification {
		t.Errorf("default SNS.SkipSignatureVerification = %v, want false", cfg.SNS.SkipSignatureVerification)
	}
	if cfg.SNS.ReplayWindow != 15*time.Minute {
		t.Errorf("default SNS.ReplayWindow = %v, want %v", cfg.SNS.ReplayWindow, 15*time.Minute)
	}
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	yaml := `
server:
  host: "127.0.0.1"
  port: 9090
database:
  url: "postgres://yaml-url"
oidc:
  discovery_url: "https://yaml-discovery"
  client_id: "yaml-client"
  client_secret: "yaml-secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)

	t.Setenv("SENDA_HOST", "192.168.1.1")
	t.Setenv("SENDA_PORT", "3000")
	t.Setenv("SENDA_DATABASE_URL", "postgres://env-url")
	t.Setenv("SENDA_OIDC_DISCOVERY_URL", "https://env-discovery")
	t.Setenv("SENDA_OIDC_CLIENT_ID", "env-client")
	t.Setenv("SENDA_OIDC_CLIENT_SECRET", "env-secret")
	t.Setenv("SENDA_MASTER_KEY", "env-master-key-that-is-at-least-32-chars!!")
	t.Setenv("SENDA_SNS_SKIP_SIGNATURE_VERIFICATION", "true")
	t.Setenv("SENDA_SNS_REPLAY_WINDOW", "45m")
	t.Setenv("SENDA_SEND_BATCH_MAX_ITEMS", "333")
	t.Setenv("SENDA_SMTP_ALLOW_INSECURE_INTERNAL_RELAY", "true")
	t.Setenv("SENDA_SMTP_TRUSTED_CLEAR_AUTH_HOSTS", "10.0.5.2,postal.internal")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("env override Server.Host = %q, want %q", cfg.Server.Host, "192.168.1.1")
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("env override Server.Port = %d, want %d", cfg.Server.Port, 3000)
	}
	if cfg.Database.URL != "postgres://env-url" {
		t.Errorf("env override Database.URL = %q", cfg.Database.URL)
	}
	if cfg.OIDC.DiscoveryURL != "https://env-discovery" {
		t.Errorf("env override OIDC.DiscoveryURL = %q", cfg.OIDC.DiscoveryURL)
	}
	if cfg.OIDC.ClientID != "env-client" {
		t.Errorf("env override OIDC.ClientID = %q", cfg.OIDC.ClientID)
	}
	if cfg.OIDC.ClientSecret != "env-secret" {
		t.Errorf("env override OIDC.ClientSecret = %q", cfg.OIDC.ClientSecret)
	}
	if cfg.Crypto.MasterKey != "env-master-key-that-is-at-least-32-chars!!" {
		t.Errorf("env override Crypto.MasterKey = %q", cfg.Crypto.MasterKey)
	}
	if !cfg.SNS.SkipSignatureVerification {
		t.Errorf("env override SNS.SkipSignatureVerification = %v, want true", cfg.SNS.SkipSignatureVerification)
	}
	if cfg.SNS.ReplayWindow != 45*time.Minute {
		t.Errorf("env override SNS.ReplayWindow = %v, want %v", cfg.SNS.ReplayWindow, 45*time.Minute)
	}
	if cfg.Send.BatchMaxItems != 333 {
		t.Errorf("env override Send.BatchMaxItems = %d, want %d", cfg.Send.BatchMaxItems, 333)
	}
	if !cfg.SMTP.AllowInsecureInternalRelay {
		t.Errorf("env override SMTP.AllowInsecureInternalRelay = %v, want true", cfg.SMTP.AllowInsecureInternalRelay)
	}
	if got, want := strings.Join(cfg.SMTP.TrustedClearAuthHosts, ","), "10.0.5.2,postal.internal"; got != want {
		t.Errorf("env override SMTP.TrustedClearAuthHosts = %q, want %q", got, want)
	}
}

func TestLoad_RequiredFieldMissing_DatabaseURL(t *testing.T) {
	yaml := `
oidc:
  discovery_url: "https://auth.example.com"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing database URL")
	}
}

func TestLoad_RequiredFieldMissing_OIDCFields(t *testing.T) {
	yaml := `
database:
  url: "postgres://localhost/senda"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing OIDC fields")
	}
}

func TestLoad_RequiredFieldMissing_MasterKey(t *testing.T) {
	yaml := `
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com"
  client_id: "client"
  client_secret: "secret"
`
	path := writeYAML(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing master key")
	}
}

func TestLoad_MasterKeyTooShort(t *testing.T) {
	yaml := `
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "short-key"
`
	path := writeYAML(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for master key < 32 chars")
	}
}

func TestLoad_ProductionAllowsMissingMetricsToken(t *testing.T) {
	yaml := `
environment: "production"
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected missing metrics token to be allowed, got %v", err)
	}
	if cfg.Server.MetricsToken != "" {
		t.Fatalf("expected empty metrics token, got %q", cfg.Server.MetricsToken)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeYAML(t, `{{{invalid yaml:::`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_ConfigExampleYAML(t *testing.T) {
	t.Setenv("SENDA_DATABASE_URL", "postgres://senda:senda@localhost:5432/senda?sslmode=disable")
	t.Setenv("SENDA_OIDC_DISCOVERY_URL", "http://localhost:9090/realms/senda/.well-known/openid-configuration")
	t.Setenv("SENDA_OIDC_CLIENT_ID", "senda-web")
	t.Setenv("SENDA_OIDC_CLIENT_SECRET", "senda-dev-secret")
	t.Setenv("SENDA_MASTER_KEY", "dev-master-key-change-in-production")

	_, err := Load("config.example.yaml")
	if err != nil {
		t.Fatalf("expected config.example.yaml to load, got error: %v", err)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	// Even with an empty YAML (only required fields), env vars override defaults
	yaml := `
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)

	t.Setenv("SENDA_HOST", "10.0.0.1")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "10.0.0.1" {
		t.Errorf("env should override default: Server.Host = %q, want %q", cfg.Server.Host, "10.0.0.1")
	}
}

func TestLoad_EnvOverridesYAML_Priority(t *testing.T) {
	// Env vars should take priority over YAML values
	yaml := `
server:
  host: "yaml-host"
database:
  url: "postgres://yaml-db"
oidc:
  discovery_url: "https://yaml-url"
  client_id: "yaml-id"
  client_secret: "yaml-secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)

	t.Setenv("SENDA_HOST", "env-host")
	t.Setenv("SENDA_DATABASE_URL", "postgres://env-db")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "env-host" {
		t.Errorf("env priority: Server.Host = %q, want %q", cfg.Server.Host, "env-host")
	}
	if cfg.Database.URL != "postgres://env-db" {
		t.Errorf("env priority: Database.URL = %q, want %q", cfg.Database.URL, "postgres://env-db")
	}
}

func TestLoad_MasterKeyExactly32Chars(t *testing.T) {
	yaml := `
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "12345678901234567890123456789012"
`
	path := writeYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("master key exactly 32 chars should be valid: %v", err)
	}
	if len(cfg.Crypto.MasterKey) != 32 {
		t.Errorf("master key length = %d, want 32", len(cfg.Crypto.MasterKey))
	}
}

func TestLoad_EmptyYAML(t *testing.T) {
	path := writeYAML(t, `{}`)

	// Provide all required fields via env vars.
	t.Setenv("SENDA_DATABASE_URL", "postgres://localhost/senda")
	t.Setenv("SENDA_OIDC_DISCOVERY_URL", "https://auth.example.com")
	t.Setenv("SENDA_OIDC_CLIENT_ID", "client")
	t.Setenv("SENDA_OIDC_CLIENT_SECRET", "secret")
	t.Setenv("SENDA_MASTER_KEY", "this-is-a-32-char-master-key!!!!")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Defaults should be applied for fields not set via env.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want default %d", cfg.Server.Port, 8080)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want default %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format = %q, want default %q", cfg.Log.Format, "json")
	}

	// Required fields set via env should be present.
	if cfg.Database.URL != "postgres://localhost/senda" {
		t.Errorf("Database.URL = %q, want %q", cfg.Database.URL, "postgres://localhost/senda")
	}
	if cfg.OIDC.DiscoveryURL != "https://auth.example.com" {
		t.Errorf("OIDC.DiscoveryURL = %q, want %q", cfg.OIDC.DiscoveryURL, "https://auth.example.com")
	}
}

func TestLoad_InvalidEnvVarValue(t *testing.T) {
	yamlContent := `
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yamlContent)

	t.Setenv("SENDA_PORT", "not-a-number")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid SENDA_PORT value")
	}

	if !strings.Contains(err.Error(), "SENDA_PORT") {
		t.Errorf("error should mention SENDA_PORT, got: %v", err)
	}
}

func TestLoad_ProductionRejectsHttpDiscoveryURL(t *testing.T) {
	yaml := `
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "http://auth.example.com/.well-known/openid-configuration"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)
	t.Setenv("SENDA_ENVIRONMENT", "production")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected production config to reject http OIDC discovery URL")
	}
	if !strings.Contains(err.Error(), "oidc.discovery_url") {
		t.Fatalf("expected error to mention oidc.discovery_url, got: %v", err)
	}
}

func TestLoad_ProductionRejectsHttpAllowedOrigins(t *testing.T) {
	yaml := `
server:
  allowed_origins:
    - "http://app.example.com"
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com/.well-known/openid-configuration"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)
	t.Setenv("SENDA_ENVIRONMENT", "production")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected production config to reject http allowed origins")
	}
	if !strings.Contains(err.Error(), "allowed_origins") {
		t.Fatalf("expected error to mention allowed_origins, got: %v", err)
	}
}

func TestLoad_ProductionRejectsSNSSkipSignatureVerification(t *testing.T) {
	yaml := `
database:
  url: "postgres://localhost/senda"
oidc:
  discovery_url: "https://auth.example.com/.well-known/openid-configuration"
  client_id: "client"
  client_secret: "secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)
	t.Setenv("SENDA_ENVIRONMENT", "production")
	t.Setenv("SENDA_SNS_SKIP_SIGNATURE_VERIFICATION", "true")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected production config to reject SNS signature verification bypass")
	}
	if !strings.Contains(err.Error(), "SENDA_SNS_SKIP_SIGNATURE_VERIFICATION") {
		t.Fatalf("expected error to mention SENDA_SNS_SKIP_SIGNATURE_VERIFICATION, got: %v", err)
	}
}

func TestLoad_ScreenshotDefaults(t *testing.T) {
	yaml := `
database:
  url: "postgres://user:pass@localhost:5432/senda"
oidc:
  discovery_url: "https://auth.example.com/.well-known/openid-configuration"
  client_id: "my-client"
  client_secret: "my-secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Screenshot.Enabled {
		t.Errorf("default Screenshot.Enabled = %v, want false", cfg.Screenshot.Enabled)
	}
	if cfg.Screenshot.ChromiumPath != "/headless-shell/headless-shell" {
		t.Errorf("default Screenshot.ChromiumPath = %q, want %q", cfg.Screenshot.ChromiumPath, "/headless-shell/headless-shell")
	}
	if cfg.Screenshot.Timeout != 15*time.Second {
		t.Errorf("default Screenshot.Timeout = %v, want %v", cfg.Screenshot.Timeout, 15*time.Second)
	}
	if cfg.Screenshot.StartupTimeout != 5*time.Second {
		t.Errorf("default Screenshot.StartupTimeout = %v, want %v", cfg.Screenshot.StartupTimeout, 5*time.Second)
	}
	if cfg.Screenshot.MaxHeightPx != 6000 {
		t.Errorf("default Screenshot.MaxHeightPx = %d, want 6000", cfg.Screenshot.MaxHeightPx)
	}
	if cfg.Screenshot.MaxConcurrent != 4 {
		t.Errorf("default Screenshot.MaxConcurrent = %d, want 4", cfg.Screenshot.MaxConcurrent)
	}
	if cfg.Screenshot.DesktopWidthPx != 1280 {
		t.Errorf("default Screenshot.DesktopWidthPx = %d, want 1280", cfg.Screenshot.DesktopWidthPx)
	}
	if cfg.Screenshot.MobileWidthPx != 390 {
		t.Errorf("default Screenshot.MobileWidthPx = %d, want 390", cfg.Screenshot.MobileWidthPx)
	}
}

func TestLoad_ScreenshotEnvOverrides(t *testing.T) {
	yaml := `
database:
  url: "postgres://user:pass@localhost:5432/senda"
oidc:
  discovery_url: "https://auth.example.com/.well-known/openid-configuration"
  client_id: "my-client"
  client_secret: "my-secret"
crypto:
  master_key: "this-is-a-32-char-master-key!!!!"
`
	path := writeYAML(t, yaml)

	t.Setenv("SENDA_SCREENSHOT_ENABLED", "true")
	t.Setenv("SENDA_SCREENSHOT_CHROMIUM_PATH", "/usr/bin/chromium")
	t.Setenv("SENDA_SCREENSHOT_TIMEOUT", "30s")
	t.Setenv("SENDA_SCREENSHOT_MAX_CONCURRENT", "8")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Screenshot.Enabled {
		t.Errorf("env override Screenshot.Enabled = %v, want true", cfg.Screenshot.Enabled)
	}
	if cfg.Screenshot.ChromiumPath != "/usr/bin/chromium" {
		t.Errorf("env override Screenshot.ChromiumPath = %q, want %q", cfg.Screenshot.ChromiumPath, "/usr/bin/chromium")
	}
	if cfg.Screenshot.Timeout != 30*time.Second {
		t.Errorf("env override Screenshot.Timeout = %v, want %v", cfg.Screenshot.Timeout, 30*time.Second)
	}
	if cfg.Screenshot.MaxConcurrent != 8 {
		t.Errorf("env override Screenshot.MaxConcurrent = %d, want 8", cfg.Screenshot.MaxConcurrent)
	}
}
