package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// Config is the root configuration for the Senda application.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	OIDC     OIDCConfig     `yaml:"oidc"`
	Crypto   CryptoConfig   `yaml:"crypto"`
	SMTP     SMTPConfig     `yaml:"smtp"`
	Log      LogConfig      `yaml:"log"`
	Tracking TrackingConfig `yaml:"tracking"`
}

type TrackingConfig struct {
	BaseURL string `yaml:"base_url" env:"SENDA_TRACKING_BASE_URL"`
}

type ServerConfig struct {
	Host            string        `yaml:"host" env:"SENDA_HOST" default:"0.0.0.0"`
	Port            int           `yaml:"port" env:"SENDA_PORT" default:"8080"`
	ReadTimeout     time.Duration `yaml:"read_timeout" default:"30s"`
	WriteTimeout    time.Duration `yaml:"write_timeout" default:"30s"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" default:"15s"`
}

type DatabaseConfig struct {
	URL             string `yaml:"url" env:"SENDA_DATABASE_URL" required:"true"`
	MaxOpenConns    int    `yaml:"max_open_conns" default:"25"`
	MaxIdleConns    int    `yaml:"max_idle_conns" default:"10"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime" default:"5m"`
	MigrateOnStart  bool   `yaml:"migrate_on_start" default:"true"`
	MigrationsPath  string `yaml:"migrations_path" env:"SENDA_MIGRATIONS_PATH" default:"migrations"`
}

type OIDCConfig struct {
	Mode         string `yaml:"mode" env:"SENDA_OIDC_MODE" default:"oidc"`
	DiscoveryURL string `yaml:"discovery_url" env:"SENDA_OIDC_DISCOVERY_URL"`
	ClientID     string `yaml:"client_id" env:"SENDA_OIDC_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" env:"SENDA_OIDC_CLIENT_SECRET"`
	TestSecret   string `yaml:"test_secret" env:"SENDA_OIDC_TEST_SECRET"`
}

type SMTPConfig struct {
	Host string `yaml:"host" env:"SENDA_SMTP_HOST"`
	Port int    `yaml:"port" env:"SENDA_SMTP_PORT" default:"1025"`
}

type CryptoConfig struct {
	MasterKey string `yaml:"master_key" env:"SENDA_MASTER_KEY" required:"true"`
}

type LogConfig struct {
	Level  string `yaml:"level" default:"info"`
	Format string `yaml:"format" default:"json"`
}

// Load reads configuration from a YAML file, applies environment variable
// overrides (SENDA_ prefix), sets defaults for missing fields, and validates
// required fields.
//
// Priority: env vars > YAML values > defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Parse into raw map to detect which keys were explicitly set in YAML.
	var raw map[string]any
	if err := yaml.Load(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	var cfg Config
	if err := yaml.Load(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(&cfg, raw)

	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("config env overrides: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

// applyDefaults walks struct fields and sets fields to their `default` tag
// value when the field was not explicitly present in the YAML source.
func applyDefaults(cfg *Config, raw map[string]any) {
	applyDefaultsToStruct(reflect.ValueOf(cfg).Elem(), raw)
}

func applyDefaultsToStruct(v reflect.Value, raw map[string]any) {
	t := v.Type()
	for i := range t.NumField() {
		field := v.Field(i)
		ft := t.Field(i)

		yamlKey := ft.Tag.Get("yaml")
		if yamlKey == "" {
			yamlKey = strings.ToLower(ft.Name)
		}

		if field.Kind() == reflect.Struct && ft.Type != reflect.TypeOf(time.Duration(0)) {
			sub, _ := raw[yamlKey].(map[string]any)
			applyDefaultsToStruct(field, sub)
			continue
		}

		defaultVal := ft.Tag.Get("default")
		if defaultVal == "" {
			continue
		}

		// Only apply default if the key was NOT present in the YAML.
		if _, present := raw[yamlKey]; present {
			continue
		}

		// Default values are compile-time constants in struct tags; parsing
		// errors here indicate a programming bug, not a user error.
		_ = setFieldFromString(field, ft.Type, defaultVal)
	}
}

// applyEnvOverrides walks struct fields and overrides values with matching
// environment variables specified in the `env` tag.
func applyEnvOverrides(cfg *Config) error {
	var errs []error
	applyEnvToStruct(reflect.ValueOf(cfg).Elem(), &errs)
	return errors.Join(errs...)
}

func applyEnvToStruct(v reflect.Value, errs *[]error) {
	t := v.Type()
	for i := range t.NumField() {
		field := v.Field(i)
		ft := t.Field(i)

		if field.Kind() == reflect.Struct && ft.Type != reflect.TypeOf(time.Duration(0)) {
			applyEnvToStruct(field, errs)
			continue
		}

		envKey := ft.Tag.Get("env")
		if envKey == "" {
			continue
		}

		envVal, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		if err := setFieldFromString(field, ft.Type, envVal); err != nil {
			*errs = append(*errs, fmt.Errorf("invalid value for %s: %w", envKey, err))
		}
	}
}

func setFieldFromString(field reflect.Value, fieldType reflect.Type, val string) error {
	switch fieldType.Kind() {
	case reflect.String:
		field.SetString(val)
	case reflect.Int:
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		field.SetInt(int64(n))
	case reflect.Int64:
		if fieldType == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(val)
			if err != nil {
				return err
			}
			field.SetInt(int64(d))
		} else {
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(n)
		}
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		field.SetBool(b)
	}
	return nil
}

// validate checks required fields and business rules.
func validate(cfg *Config) error {
	var errs []error

	validateRequired(reflect.ValueOf(cfg).Elem(), "", &errs)

	if cfg.Database.URL != "" &&
		!strings.HasPrefix(cfg.Database.URL, "postgres://") &&
		!strings.HasPrefix(cfg.Database.URL, "postgresql://") {
		errs = append(errs, fmt.Errorf("database.url must start with postgres:// or postgresql://"))
	}

	// OIDC validation depends on mode.
	switch cfg.OIDC.Mode {
	case "test":
		if cfg.OIDC.TestSecret == "" {
			errs = append(errs, fmt.Errorf("oidc.test_secret is required when mode is \"test\""))
		}
	case "dual":
		// Dual mode needs both real OIDC fields AND the test secret.
		if cfg.OIDC.DiscoveryURL == "" {
			errs = append(errs, fmt.Errorf("oidc.discovery_url is required when mode is \"dual\""))
		}
		if cfg.OIDC.ClientID == "" {
			errs = append(errs, fmt.Errorf("oidc.client_id is required when mode is \"dual\""))
		}
		if cfg.OIDC.TestSecret == "" {
			errs = append(errs, fmt.Errorf("oidc.test_secret is required when mode is \"dual\""))
		}
	default: // "oidc" or any other value → require real OIDC fields.
		if cfg.OIDC.DiscoveryURL == "" {
			errs = append(errs, fmt.Errorf("oidc.discovery_url is required when mode is %q", cfg.OIDC.Mode))
		}
		if cfg.OIDC.ClientID == "" {
			errs = append(errs, fmt.Errorf("oidc.client_id is required when mode is %q", cfg.OIDC.Mode))
		}
		if cfg.OIDC.ClientSecret == "" {
			errs = append(errs, fmt.Errorf("oidc.client_secret is required when mode is %q", cfg.OIDC.Mode))
		}
	}

	if cfg.OIDC.DiscoveryURL != "" &&
		!strings.HasPrefix(cfg.OIDC.DiscoveryURL, "https://") &&
		!strings.HasPrefix(cfg.OIDC.DiscoveryURL, "http://") {
		errs = append(errs, fmt.Errorf("oidc.discovery_url must start with http:// or https://"))
	}

	if cfg.Crypto.MasterKey != "" && len(cfg.Crypto.MasterKey) < 32 {
		errs = append(errs, fmt.Errorf("crypto.master_key must be at least 32 characters, got %d", len(cfg.Crypto.MasterKey)))
	}

	return errors.Join(errs...)
}

func validateRequired(v reflect.Value, prefix string, errs *[]error) {
	t := v.Type()
	for i := range t.NumField() {
		field := v.Field(i)
		ft := t.Field(i)

		yamlName := ft.Tag.Get("yaml")
		if yamlName == "" {
			yamlName = strings.ToLower(ft.Name)
		}
		fullName := yamlName
		if prefix != "" {
			fullName = prefix + "." + yamlName
		}

		if field.Kind() == reflect.Struct && ft.Type != reflect.TypeOf(time.Duration(0)) {
			validateRequired(field, fullName, errs)
			continue
		}

		if ft.Tag.Get("required") == "true" && field.IsZero() {
			*errs = append(*errs, fmt.Errorf("%s is required", fullName))
		}
	}
}
