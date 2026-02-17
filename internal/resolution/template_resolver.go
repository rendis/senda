package resolution

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// ResolvedTemplate holds the fully resolved template with its version and optional locale.
type ResolvedTemplate struct {
	Template     *domain.Template
	Version      *domain.TemplateVersion
	Locale       *domain.TemplateVersionLocale // nil = use version's default
	TemplateType *domain.TemplateType
}

// TemplateResolver resolves a template by type slug through the resolution chain.
type TemplateResolver struct {
	store         port.TemplateStore
	chainResolver *ChainResolver
}

// NewTemplateResolver creates a TemplateResolver with the given dependencies.
func NewTemplateResolver(store port.TemplateStore, cr *ChainResolver) *TemplateResolver {
	return &TemplateResolver{
		store:         store,
		chainResolver: cr,
	}
}

// Resolve finds the best-matching template for the given workspace and type slug,
// applying the resolution chain and optional locale fallback.
func (r *TemplateResolver) Resolve(ctx context.Context, workspaceID uuid.UUID, typeSlug string, locale *string) (*ResolvedTemplate, error) {
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

	return &ResolvedTemplate{
		Template:     template,
		Version:      version,
		Locale:       localeContent,
		TemplateType: templateType,
	}, nil
}
