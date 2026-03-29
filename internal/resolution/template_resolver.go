package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

const templateCacheTTL = 5 * time.Minute

// ResolvedTemplate holds the fully resolved template with its version and optional locale.
type ResolvedTemplate struct {
	Template     *domain.Template              `json:"template"`
	Version      *domain.TemplateVersion       `json:"version"`
	Locale       *domain.TemplateVersionLocale  `json:"locale,omitempty"` // nil = use version's default
	TemplateType *domain.TemplateType           `json:"template_type"`
}

// TemplateResolver resolves a template by type slug through the resolution chain.
// It requires the full port.TemplateStore because it uses methods across all sub-interfaces:
//   - TemplateTypeStore: GetTypeBySlug
//   - TemplateVersionStore: GetPublishedVersion
//   - LocaleStore: GetLocale
//   - Core TemplateStore: ResolveTemplate
type TemplateResolver struct {
	store         port.TemplateStore
	cache         port.Cache
	chainResolver *ChainResolver
}

// NewTemplateResolver creates a TemplateResolver with the given dependencies.
func NewTemplateResolver(store port.TemplateStore, cache port.Cache, cr *ChainResolver) *TemplateResolver {
	return &TemplateResolver{
		store:         store,
		cache:         cache,
		chainResolver: cr,
	}
}

// Resolve finds the best-matching template for the given workspace and type slug,
// applying the resolution chain and optional locale fallback.
// Results are cached by workspace+typeSlug+locale with a 5-minute TTL.
func (r *TemplateResolver) Resolve(ctx context.Context, workspaceID uuid.UUID, typeSlug string, locale *string) (*ResolvedTemplate, error) {
	cacheKey := resolvedTemplateCacheKey(workspaceID, typeSlug, locale)

	// Try cache first.
	if data, err := r.cache.Get(ctx, cacheKey); err == nil {
		var cached ResolvedTemplate
		if err := json.Unmarshal(data, &cached); err == nil {
			return &cached, nil
		}
		slog.Warn("template cache unmarshal failed, falling through to DB", "key", cacheKey)
	}

	chain, err := r.chainResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	templateType, err := r.store.GetTypeBySlug(ctx, typeSlug, chain.Scopes)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrTemplateTypeNotFound, typeSlug)
	}

	template, err := r.store.ResolveTemplate(ctx, templateType.ID, chain.Scopes)
	if err != nil {
		return nil, fmt.Errorf("%w: type %s", domain.ErrTemplateNotFound, typeSlug)
	}

	if template.IsDisabled {
		return nil, domain.ErrTemplateDisabled
	}

	version, err := r.store.GetPublishedVersion(ctx, template.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: template %s", domain.ErrNoPublishedVersion, template.ID)
	}

	var localeContent *domain.TemplateVersionLocale

	if locale != nil && *locale != version.DefaultLocale {
		// Try exact locale match
		loc, err := r.store.GetLocale(ctx, version.ID, *locale)
		if err == nil {
			localeContent = loc
		} else if idx := strings.IndexByte(*locale, '-'); idx > 0 {
			// Try language prefix fallback (e.g., "es-CO" -> "es")
			prefix := (*locale)[:idx]
			loc, err := r.store.GetLocale(ctx, version.ID, prefix)
			if err == nil {
				localeContent = loc
			}
		}
		// On total miss, localeContent stays nil -> use version's default
	}

	resolved := &ResolvedTemplate{
		Template:     template,
		Version:      version,
		Locale:       localeContent,
		TemplateType: templateType,
	}

	// Best-effort cache write.
	if data, err := json.Marshal(resolved); err == nil {
		_ = r.cache.Set(ctx, cacheKey, data, templateCacheTTL)
	}

	return resolved, nil
}

// resolvedTemplateCacheKey builds the cache key for a resolved template.
func resolvedTemplateCacheKey(workspaceID uuid.UUID, typeSlug string, locale *string) string {
	loc := "_default"
	if locale != nil {
		loc = *locale
	}
	return fmt.Sprintf("resolved_template:%s:%s:%s", workspaceID.String(), typeSlug, loc)
}
