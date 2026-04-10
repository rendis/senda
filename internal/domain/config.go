package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rendis/senda/pkg/slug"
)

var headerNamePattern = regexp.MustCompile(`^[!#$%&'*+.^_` + "`" + `{|}~0-9A-Za-z-]+$`)

type ExternalIntegrationCapabilities struct {
	ListTemplates   bool
	ViewVersions    bool
	EditVersions    bool
	PublishVersions bool
	TestSend        bool
	BuilderAccess   bool
	MetadataAccess  bool
	LocaleAccess    bool
}

type ExternalIntegrationProfile struct {
	Slug            string
	Name            string
	Description     string
	Enabled         bool
	AuthMethodName  string
	ResolverName    string
	AllowedOrigins  []string
	AllowedHeaders  []string
	RequiredHeaders []string
	Capabilities    ExternalIntegrationCapabilities
}

func (p ExternalIntegrationProfile) Normalize() ExternalIntegrationProfile {
	p.Slug = strings.TrimSpace(strings.ToLower(p.Slug))
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	p.AuthMethodName = strings.TrimSpace(p.AuthMethodName)
	p.ResolverName = strings.TrimSpace(p.ResolverName)
	p.AllowedOrigins = normalizeAndDedupeStrings(p.AllowedOrigins, false)
	p.AllowedHeaders = normalizeAndDedupeStrings(p.AllowedHeaders, true)
	p.RequiredHeaders = normalizeAndDedupeStrings(p.RequiredHeaders, true)
	return p
}

func (p ExternalIntegrationProfile) Validate() error {
	normalized := p.Normalize()

	if err := slug.Validate(normalized.Slug); err != nil {
		return fmt.Errorf("slug: %w", err)
	}
	if normalized.Name == "" {
		return errors.New("name is required")
	}
	if normalized.AuthMethodName == "" {
		return errors.New("auth_method_name is required")
	}
	if err := validateExternalMethodName(normalized.AuthMethodName); err != nil {
		return fmt.Errorf("auth_method_name: %w", err)
	}
	if normalized.ResolverName == "" {
		return errors.New("resolver_name is required")
	}
	if err := validateExternalMethodName(normalized.ResolverName); err != nil {
		return fmt.Errorf("resolver_name: %w", err)
	}
	for _, origin := range normalized.AllowedOrigins {
		if err := validateAllowedOrigin(origin); err != nil {
			return fmt.Errorf("allowed_origin %q: %w", origin, err)
		}
	}

	allowed := make(map[string]struct{}, len(normalized.AllowedHeaders))
	for _, header := range normalized.AllowedHeaders {
		if err := validateAllowedHeaderName(header); err != nil {
			return fmt.Errorf("allowed_header %q: %w", header, err)
		}
		allowed[header] = struct{}{}
	}
	for _, header := range normalized.RequiredHeaders {
		if err := validateAllowedHeaderName(header); err != nil {
			return fmt.Errorf("required_header %q: %w", header, err)
		}
		if _, ok := allowed[header]; !ok {
			return fmt.Errorf("required header %q must be present in allowed_headers", header)
		}
	}

	return nil
}

type GlobalConfig struct {
	DefaultRetryCount              int
	RetryBackoffBaseSeconds        int
	LogRetentionDays               int
	BounceAlertThresholdPercent    float64
	ComplaintAlertThresholdPercent float64
	DomainRecheckIntervalHours     int
	OnboardingCompleted            bool
	ExternalIntegrations           []ExternalIntegrationProfile
	UpdatedAt                      time.Time
}

func (cfg *GlobalConfig) Validate() error {
	if cfg == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(cfg.ExternalIntegrations))
	for i, profile := range cfg.ExternalIntegrations {
		normalized := profile.Normalize()
		if err := normalized.Validate(); err != nil {
			return fmt.Errorf("external_integrations[%d]: %w", i, err)
		}
		if _, ok := seen[normalized.Slug]; ok {
			return fmt.Errorf("external_integrations[%d]: duplicate slug %q", i, normalized.Slug)
		}
		seen[normalized.Slug] = struct{}{}
	}

	return nil
}

func NormalizeExternalIntegrationProfiles(profiles []ExternalIntegrationProfile) []ExternalIntegrationProfile {
	out := make([]ExternalIntegrationProfile, len(profiles))
	for i, profile := range profiles {
		out[i] = profile.Normalize()
	}
	return out
}

func normalizeAndDedupeStrings(values []string, lower bool) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func validateExternalMethodName(value string) error {
	if err := slug.Validate(strings.ToLower(value)); err != nil {
		return err
	}
	return nil
}

func validateAllowedOrigin(origin string) error {
	parsed, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("invalid origin: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return errors.New("host is required")
	}
	if parsed.User != nil {
		return errors.New("userinfo is not allowed")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return errors.New("origin must not include a path")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("origin must not include query or fragment")
	}
	if origin != parsed.Scheme+"://"+parsed.Host && origin != parsed.Scheme+"://"+parsed.Host+"/" {
		return errors.New("origin must be a bare origin")
	}
	return nil
}

func validateAllowedHeaderName(value string) error {
	if !headerNamePattern.MatchString(value) {
		return errors.New("invalid header name")
	}
	lower := strings.ToLower(value)
	if lower == "host" || lower == "origin" || lower == "cookie" || lower == "authorization" {
		return errors.New("header is not allowed for external integration auth")
	}
	if strings.HasPrefix(lower, "x-forwarded-") || strings.HasPrefix(lower, "sec-") {
		return errors.New("header is not allowed for external integration auth")
	}
	return nil
}
