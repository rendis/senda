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
database:
  url: "postgres://user:pass@localhost:5432/senda"
  max_open_conns: 50
  max_idle_conns: 20
  conn_max_lifetime: "10m"
  migrate_on_start: false
oidc:
  discovery_url: "https://auth.example.com/.well-known/openid-configuration"
  client_id: "my-client"
  client_secret: "my-secret"
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

	// Database
	if cfg.Database.URL != "postgres://user:pass@localhost:5432/senda" {
		t.Errorf("Database.URL = %q, want postgres URL", cfg.Database.URL)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Errorf("Database.MaxOpenConns = %d, want 50", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 20 {
		t.Errorf("Database.MaxIdleConns = %d, want 20", cfg.Database.MaxIdleConns)
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
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("default Database.MaxOpenConns = %d, want 25", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 10 {
		t.Errorf("default Database.MaxIdleConns = %d, want 10", cfg.Database.MaxIdleConns)
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

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeYAML(t, `{{{invalid yaml:::`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
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
