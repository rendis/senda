package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/pkg/apperr"
)

// TemplateRepo implements port.TemplateStore using PostgreSQL.
type TemplateRepo struct {
	pool *pgxpool.Pool
}

// NewTemplateRepo creates a new TemplateRepo.
func NewTemplateRepo(pool *pgxpool.Pool) *TemplateRepo {
	return &TemplateRepo{pool: pool}
}

// --- Template Types ---

func (r *TemplateRepo) CreateType(ctx context.Context, tt *domain.TemplateType) error { //nolint:dupl // structurally similar to AdapterRepo.Create
	row := r.pool.QueryRow(ctx,
		`INSERT INTO template_types (id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema)
		 VALUES (@id, @slug, @name, @description, @workspace_id, @adapter_id, @sender_identity_id, @variable_schema)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":                 tt.ID,
			"slug":               tt.Slug,
			"name":               tt.Name,
			"description":        tt.Description,
			"workspace_id":       tt.WorkspaceID,
			"adapter_id":         tt.AdapterID,
			"sender_identity_id": tt.SenderIdentityID,
			"variable_schema":    coalesceJSON(tt.VariableSchema),
		},
	)

	if err := row.Scan(&tt.CreatedAt, &tt.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting template type: %w", err)
	}

	return nil
}

func (r *TemplateRepo) UpdateType(ctx context.Context, tt *domain.TemplateType) error {
	row := r.pool.QueryRow(ctx,
		`UPDATE template_types
		 SET slug = @slug, name = @name, adapter_id = @adapter_id, sender_identity_id = @sender_identity_id, updated_at = now()
		 WHERE id = @id AND deleted_at IS NULL
		 RETURNING updated_at`,
		pgx.NamedArgs{
			"id":                 tt.ID,
			"slug":               tt.Slug,
			"name":               tt.Name,
			"adapter_id":         tt.AdapterID,
			"sender_identity_id": tt.SenderIdentityID,
		},
	)

	if err := row.Scan(&tt.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("updating template type: %w", err)
	}

	return nil
}

func (r *TemplateRepo) SoftDeleteType(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE template_types SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting template type: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("template type %s not found", id)
	}
	return nil
}

func (r *TemplateRepo) GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error) {
	scopes, includeGlobal := splitChain(chain)

	// Build an array parameter for chain priority ordering.
	// The chain is ordered from most specific to least specific.
	// We use array_position to pick the most specific match.
	chainArr := make([]any, len(chain))
	for i, entry := range chain {
		if entry.Valid {
			chainArr[i] = entry.UUID
		} else {
			chainArr[i] = nil
		}
	}

	row := r.pool.QueryRow(ctx,
		`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema,
		        created_at, updated_at, deleted_at
		 FROM template_types
		 WHERE slug = @slug
		   AND (workspace_id = ANY(@scopes) OR (@include_global::bool AND workspace_id IS NULL))
		   AND deleted_at IS NULL
		 ORDER BY
		   CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END,
		   id DESC
		 LIMIT 1`,
		pgx.NamedArgs{
			"slug":           slug,
			"scopes":         scopes,
			"include_global": includeGlobal,
		},
	)

	return scanTemplateType(row)
}

func (r *TemplateRepo) FindTypeBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error) {
	var row pgx.Row
	if wsID == nil {
		row = r.pool.QueryRow(ctx,
			`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema,
			        created_at, updated_at, deleted_at
			 FROM template_types
			 WHERE slug = @slug AND workspace_id IS NULL AND deleted_at IS NULL`,
			pgx.NamedArgs{"slug": slug},
		)
	} else {
		row = r.pool.QueryRow(ctx,
			`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema,
			        created_at, updated_at, deleted_at
			 FROM template_types
			 WHERE slug = @slug AND workspace_id = @workspace_id AND deleted_at IS NULL`,
			pgx.NamedArgs{"slug": slug, "workspace_id": *wsID},
		)
	}

	return scanTemplateType(row)
}

func (r *TemplateRepo) ListTypes(ctx context.Context, wsID *uuid.UUID, opts port.ListOptions) ([]*domain.TemplateType, string, error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, "", err
	}

	fetchLimit := limit + 1

	var rows pgx.Rows
	if wsID == nil {
		// Global scope: only global types.
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema,
				        created_at, updated_at, deleted_at
				 FROM template_types
				 WHERE workspace_id IS NULL AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema,
				        created_at, updated_at, deleted_at
				 FROM template_types
				 WHERE workspace_id IS NULL AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"limit": fetchLimit},
			)
		}
	} else {
		// Workspace scope: types in this workspace.
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema,
				        created_at, updated_at, deleted_at
				 FROM template_types
				 WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"workspace_id": *wsID, "after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema,
				        created_at, updated_at, deleted_at
				 FROM template_types
				 WHERE workspace_id = @workspace_id AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"workspace_id": *wsID, "limit": fetchLimit},
			)
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing template types: %w", err)
	}

	types, err := pgx.CollectRows(rows, scanTemplateTypeRow)
	if err != nil {
		return nil, "", fmt.Errorf("collecting template types: %w", err)
	}

	var nextCursor string
	if len(types) > limit {
		types = types[:limit]
		nextCursor = EncodeCursor(types[limit-1].ID)
	}

	return types, nextCursor, nil
}

// --- Templates ---

func (r *TemplateRepo) CreateTemplate(ctx context.Context, tpl *domain.Template) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO templates (id, template_type_id, workspace_id, is_disabled)
		 VALUES (@id, @template_type_id, @workspace_id, @is_disabled)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":               tpl.ID,
			"template_type_id": tpl.TemplateTypeID,
			"workspace_id":     tpl.WorkspaceID,
			"is_disabled":      tpl.IsDisabled,
		},
	)

	if err := row.Scan(&tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting template: %w", err)
	}

	return nil
}

func (r *TemplateRepo) GetTemplateByID(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
		 FROM templates
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	return scanTemplate(row)
}

func (r *TemplateRepo) GetTypeByID(ctx context.Context, id uuid.UUID) (*domain.TemplateType, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, slug, name, description, workspace_id, adapter_id, sender_identity_id, variable_schema, created_at, updated_at, deleted_at
		 FROM template_types
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	return scanTemplateType(row)
}

func (r *TemplateRepo) GetByTypeAndScope(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error) {
	var row pgx.Row
	if wsID == nil {
		row = r.pool.QueryRow(ctx,
			`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
			 FROM templates
			 WHERE template_type_id = @type_id AND workspace_id IS NULL AND deleted_at IS NULL`,
			pgx.NamedArgs{"type_id": typeID},
		)
	} else {
		row = r.pool.QueryRow(ctx,
			`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
			 FROM templates
			 WHERE template_type_id = @type_id AND workspace_id = @workspace_id AND deleted_at IS NULL`,
			pgx.NamedArgs{"type_id": typeID, "workspace_id": *wsID},
		)
	}

	return scanTemplate(row)
}

func (r *TemplateRepo) ListByType(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID, opts port.ListOptions) ([]*domain.Template, string, error) {
	limit, afterID, err := ApplyPagination(opts)
	if err != nil {
		return nil, "", err
	}

	fetchLimit := limit + 1

	var rows pgx.Rows
	if wsID == nil {
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
				 FROM templates
				 WHERE template_type_id = @type_id AND workspace_id IS NULL AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"type_id": typeID, "after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
				 FROM templates
				 WHERE template_type_id = @type_id AND workspace_id IS NULL AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"type_id": typeID, "limit": fetchLimit},
			)
		}
	} else {
		if afterID != nil {
			rows, err = r.pool.Query(ctx,
				`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
				 FROM templates
				 WHERE template_type_id = @type_id AND workspace_id = @workspace_id AND deleted_at IS NULL AND id < @after_id
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"type_id": typeID, "workspace_id": *wsID, "after_id": *afterID, "limit": fetchLimit},
			)
		} else {
			rows, err = r.pool.Query(ctx,
				`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
				 FROM templates
				 WHERE template_type_id = @type_id AND workspace_id = @workspace_id AND deleted_at IS NULL
				 ORDER BY id DESC
				 LIMIT @limit`,
				pgx.NamedArgs{"type_id": typeID, "workspace_id": *wsID, "limit": fetchLimit},
			)
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("listing templates by type: %w", err)
	}

	templates, err := pgx.CollectRows(rows, scanTemplateRow)
	if err != nil {
		return nil, "", fmt.Errorf("collecting templates: %w", err)
	}

	var nextCursor string
	if len(templates) > limit {
		templates = templates[:limit]
		nextCursor = EncodeCursor(templates[limit-1].ID)
	}

	return templates, nextCursor, nil
}

func (r *TemplateRepo) ResolveTemplate(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error) {
	scopes, includeGlobal := splitChain(chain)

	row := r.pool.QueryRow(ctx,
		`SELECT id, template_type_id, workspace_id, is_disabled, created_at, updated_at, deleted_at
		 FROM templates
		 WHERE template_type_id = @type_id
		   AND (workspace_id = ANY(@scopes) OR (@include_global::bool AND workspace_id IS NULL))
		   AND deleted_at IS NULL
		 ORDER BY
		   CASE WHEN workspace_id IS NULL THEN 1 ELSE 0 END,
		   id DESC
		 LIMIT 1`,
		pgx.NamedArgs{
			"type_id":        typeID,
			"scopes":         scopes,
			"include_global": includeGlobal,
		},
	)

	return scanTemplate(row)
}

func (r *TemplateRepo) SetDisabled(ctx context.Context, templateID uuid.UUID, wsID *uuid.UUID, disabled bool) error {
	var (
		tag pgconn.CommandTag
		err error
	)

	if wsID == nil {
		tag, err = r.pool.Exec(ctx,
			`UPDATE templates
			 SET is_disabled = @is_disabled, updated_at = now()
			 WHERE id = @id AND workspace_id IS NULL AND deleted_at IS NULL`,
			pgx.NamedArgs{
				"id":          templateID,
				"is_disabled": disabled,
			},
		)
	} else {
		tag, err = r.pool.Exec(ctx,
			`UPDATE templates
			 SET is_disabled = @is_disabled, updated_at = now()
			 WHERE id = @id AND workspace_id = @workspace_id AND deleted_at IS NULL`,
			pgx.NamedArgs{
				"id":           templateID,
				"workspace_id": *wsID,
				"is_disabled":  disabled,
			},
		)
	}

	if err != nil {
		return fmt.Errorf("updating template disabled state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("template %s not found in scope", templateID)
	}

	return nil
}

func (r *TemplateRepo) SoftDeleteTemplate(ctx context.Context, templateID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE templates SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": templateID},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("template %s not found", templateID)
	}
	return nil
}

func (r *TemplateRepo) DeleteDraftVersion(ctx context.Context, versionID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM template_versions WHERE id = @id AND status = 'draft'`,
		pgx.NamedArgs{"id": versionID},
	)
	if err != nil {
		return fmt.Errorf("deleting draft version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("draft version %s not found", versionID)
	}
	return nil
}

// --- Versions ---

func (r *TemplateRepo) CreateVersion(ctx context.Context, ver *domain.TemplateVersion) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var lockedTemplateID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT id FROM templates WHERE id = @template_id FOR UPDATE`,
		pgx.NamedArgs{"template_id": ver.TemplateID},
	).Scan(&lockedTemplateID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("template %s not found", ver.TemplateID)
		}
		return fmt.Errorf("locking template for version creation: %w", err)
	}

	// Auto-calculate version number
	var versionNumber int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version_number), 0) + 1
			 FROM template_versions
		 WHERE template_id = @template_id`,
		pgx.NamedArgs{"template_id": ver.TemplateID},
	).Scan(&versionNumber)
	if err != nil {
		return fmt.Errorf("calculating version number: %w", err)
	}
	ver.VersionNumber = versionNumber

	row := tx.QueryRow(ctx,
		`INSERT INTO template_versions (id, template_id, version_number, status, subject, preview_text,
			                                from_name, reply_to, body_mjml, default_locale,
			                                editor_data, created_by)
			 VALUES (@id, @template_id, @version_number, @status, @subject, @preview_text,
			         @from_name, @reply_to, @body_mjml, @default_locale,
			         @editor_data, @created_by)
			 RETURNING version_number, created_at, updated_at`,
		pgx.NamedArgs{
			"id":             ver.ID,
			"template_id":    ver.TemplateID,
			"version_number": versionNumber,
			"status":         ver.Status,
			"subject":        ver.Subject,
			"preview_text":   ver.PreviewText,
			"from_name":      ver.FromName,
			"reply_to":       ver.ReplyTo,
			"body_mjml":      ver.BodyMJML,
			"default_locale": ver.DefaultLocale,
			"editor_data":    ver.EditorData,
			"created_by":     ver.CreatedBy,
		},
	)

	if err := row.Scan(&ver.VersionNumber, &ver.CreatedAt, &ver.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting template version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (r *TemplateRepo) CloneVersion(
	ctx context.Context,
	templateID, sourceVersionID uuid.UUID,
	createdBy *uuid.UUID,
) (*domain.TemplateVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := lockTemplateForVersionMutation(ctx, tx, templateID, "clone"); err != nil {
		return nil, err
	}

	sourceVersion, err := loadTemplateVersionForClone(ctx, tx, templateID, sourceVersionID)
	if err != nil {
		return nil, err
	}

	sourceLocales, err := listTemplateVersionLocalesForClone(ctx, tx, sourceVersionID)
	if err != nil {
		return nil, err
	}

	cloned := &domain.TemplateVersion{
		ID:            uuid.Must(uuid.NewV7()),
		TemplateID:    templateID,
		Status:        domain.VersionStatusDraft,
		Subject:       sourceVersion.Subject,
		PreviewText:   sourceVersion.PreviewText,
		FromName:      sourceVersion.FromName,
		ReplyTo:       sourceVersion.ReplyTo,
		BodyMJML:      sourceVersion.BodyMJML,
		DefaultLocale: sourceVersion.DefaultLocale,
		EditorData:    sourceVersion.EditorData,
		CreatedBy:     createdBy,
	}

	if err := insertClonedVersion(ctx, tx, cloned); err != nil {
		return nil, err
	}

	if err := insertClonedLocales(ctx, tx, cloned.ID, sourceLocales); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing cloned template version: %w", err)
	}

	return cloned, nil
}

func lockTemplateForVersionMutation(ctx context.Context, tx pgx.Tx, templateID uuid.UUID, operation string) error {
	var lockedTemplateID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT id FROM templates WHERE id = @template_id FOR UPDATE`,
		pgx.NamedArgs{"template_id": templateID},
	).Scan(&lockedTemplateID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("template %s not found", templateID)
		}
		return fmt.Errorf("locking template for version %s: %w", operation, err)
	}
	return nil
}

func loadTemplateVersionForClone(
	ctx context.Context,
	tx pgx.Tx,
	templateID, sourceVersionID uuid.UUID,
) (*domain.TemplateVersion, error) {
	row := tx.QueryRow(ctx,
		`SELECT id, template_id, version_number, status, subject, preview_text,
		        from_name, reply_to, body_mjml, default_locale,
		        editor_data, created_by, published_at, archived_at, created_at, updated_at
		 FROM template_versions
		 WHERE id = @source_version_id AND template_id = @template_id`,
		pgx.NamedArgs{
			"source_version_id": sourceVersionID,
			"template_id":       templateID,
		},
	)
	return scanTemplateVersion(row)
}

func listTemplateVersionLocalesForClone(
	ctx context.Context,
	tx pgx.Tx,
	sourceVersionID uuid.UUID,
) ([]*domain.TemplateVersionLocale, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, template_version_id, locale, subject, preview_text, from_name,
		        body_mjml, editor_data, created_at, updated_at
		 FROM template_version_locales
		 WHERE template_version_id = @source_version_id
		 ORDER BY created_at ASC`,
		pgx.NamedArgs{"source_version_id": sourceVersionID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing source locales for clone: %w", err)
	}

	locales, err := pgx.CollectRows(rows, scanTemplateVersionLocaleRow)
	if err != nil {
		return nil, fmt.Errorf("collecting source locales for clone: %w", err)
	}
	return locales, nil
}

func insertClonedVersion(ctx context.Context, tx pgx.Tx, cloned *domain.TemplateVersion) error {
	versionNumber, err := nextTemplateVersionNumber(ctx, tx, cloned.TemplateID)
	if err != nil {
		return err
	}
	cloned.VersionNumber = versionNumber

	if err := tx.QueryRow(ctx,
		`INSERT INTO template_versions (id, template_id, version_number, status, subject, preview_text,
		                                from_name, reply_to, body_mjml, default_locale,
		                                editor_data, created_by)
		 VALUES (@id, @template_id, @version_number, @status, @subject, @preview_text,
		         @from_name, @reply_to, @body_mjml, @default_locale,
		         @editor_data, @created_by)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":             cloned.ID,
			"template_id":    cloned.TemplateID,
			"version_number": cloned.VersionNumber,
			"status":         cloned.Status,
			"subject":        cloned.Subject,
			"preview_text":   cloned.PreviewText,
			"from_name":      cloned.FromName,
			"reply_to":       cloned.ReplyTo,
			"body_mjml":      cloned.BodyMJML,
			"default_locale": cloned.DefaultLocale,
			"editor_data":    cloned.EditorData,
			"created_by":     cloned.CreatedBy,
		},
	).Scan(&cloned.CreatedAt, &cloned.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting cloned template version: %w", err)
	}
	return nil
}

func nextTemplateVersionNumber(ctx context.Context, tx pgx.Tx, templateID uuid.UUID) (int, error) {
	var versionNumber int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version_number), 0) + 1
		 FROM template_versions
		 WHERE template_id = @template_id`,
		pgx.NamedArgs{"template_id": templateID},
	).Scan(&versionNumber); err != nil {
		return 0, fmt.Errorf("calculating cloned version number: %w", err)
	}
	return versionNumber, nil
}

func insertClonedLocales(
	ctx context.Context,
	tx pgx.Tx,
	clonedVersionID uuid.UUID,
	sourceLocales []*domain.TemplateVersionLocale,
) error {
	for _, sourceLocale := range sourceLocales {
		if _, err := tx.Exec(ctx,
			`INSERT INTO template_version_locales (id, template_version_id, locale, subject, preview_text,
			                                       from_name, body_mjml, editor_data)
			 VALUES (@id, @template_version_id, @locale, @subject, @preview_text,
			         @from_name, @body_mjml, @editor_data)`,
			pgx.NamedArgs{
				"id":                  uuid.Must(uuid.NewV7()),
				"template_version_id": clonedVersionID,
				"locale":              sourceLocale.Locale,
				"subject":             sourceLocale.Subject,
				"preview_text":        sourceLocale.PreviewText,
				"from_name":           sourceLocale.FromName,
				"body_mjml":           sourceLocale.BodyMJML,
				"editor_data":         sourceLocale.EditorData,
			},
		); err != nil {
			if appErr := classifyPgError(err); appErr != nil {
				return appErr
			}
			return fmt.Errorf("inserting cloned template locale %s: %w", sourceLocale.Locale, err)
		}
	}
	return nil
}

func (r *TemplateRepo) GetVersionByID(ctx context.Context, versionID uuid.UUID) (*domain.TemplateVersion, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, template_id, version_number, status, subject, preview_text,
		        from_name, reply_to, body_mjml, default_locale,
		        editor_data, created_by, published_at, archived_at, created_at, updated_at
		 FROM template_versions
		 WHERE id = @id`,
		pgx.NamedArgs{"id": versionID},
	)

	return scanTemplateVersion(row)
}

func (r *TemplateRepo) UpdateVersion(ctx context.Context, ver *domain.TemplateVersion) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE template_versions
		 SET subject = @subject, preview_text = @preview_text,
		     from_name = @from_name, reply_to = @reply_to,
		     body_mjml = @body_mjml, default_locale = @default_locale,
		     editor_data = @editor_data, updated_at = now()
		 WHERE id = @id AND status = 'draft'`,
		pgx.NamedArgs{
			"id":             ver.ID,
			"subject":        ver.Subject,
			"preview_text":   ver.PreviewText,
			"from_name":      ver.FromName,
			"reply_to":       ver.ReplyTo,
			"body_mjml":      ver.BodyMJML,
			"default_locale": ver.DefaultLocale,
			"editor_data":    ver.EditorData,
		},
	)
	if err != nil {
		return fmt.Errorf("updating template version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("template version not found or not in draft status")
	}

	return nil
}

func (r *TemplateRepo) GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, template_id, version_number, status, subject, preview_text,
		        from_name, reply_to, body_mjml, default_locale,
		        editor_data, created_by, published_at, archived_at, created_at, updated_at
		 FROM template_versions
		 WHERE template_id = @template_id AND status = 'published'`,
		pgx.NamedArgs{"template_id": templateID},
	)

	return scanTemplateVersion(row)
}

func (r *TemplateRepo) Publish(ctx context.Context, versionID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Get the template_id for this version
	var templateID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT template_id FROM template_versions WHERE id = @id`,
		pgx.NamedArgs{"id": versionID},
	).Scan(&templateID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("template version %s not found", versionID)
		}
		return fmt.Errorf("looking up template version: %w", err)
	}

	// Lock the template row to prevent concurrent publishes
	var lockedID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT id FROM templates WHERE id = @template_id FOR UPDATE`,
		pgx.NamedArgs{"template_id": templateID},
	).Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("template %s not found", templateID)
		}
		return fmt.Errorf("lock template: %w", err)
	}

	// Archive existing published version for this template
	_, err = tx.Exec(ctx,
		`UPDATE template_versions
		 SET status = 'archived', archived_at = now(), updated_at = now()
		 WHERE template_id = @template_id AND status = 'published'`,
		pgx.NamedArgs{"template_id": templateID},
	)
	if err != nil {
		return fmt.Errorf("archiving previous version: %w", err)
	}

	// Publish the target version
	tag, err := tx.Exec(ctx,
		`UPDATE template_versions
		 SET status = 'published', published_at = now(), updated_at = now()
		 WHERE id = @id AND status = 'draft'`,
		pgx.NamedArgs{"id": versionID},
	)
	if err != nil {
		return fmt.Errorf("publishing version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.Conflict("version %s is not in draft status", versionID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing publish: %w", err)
	}

	return nil
}

func (r *TemplateRepo) GetLatestVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, template_id, version_number, status, subject, preview_text,
		        from_name, reply_to, body_mjml, default_locale,
		        editor_data, created_by, published_at, archived_at, created_at, updated_at
		 FROM template_versions
		 WHERE template_id = @template_id
		 ORDER BY version_number DESC
		 LIMIT 1`,
		pgx.NamedArgs{"template_id": templateID},
	)

	return scanTemplateVersion(row)
}

func (r *TemplateRepo) ListVersions(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, template_id, version_number, status, subject, preview_text,
		        from_name, reply_to, body_mjml, default_locale,
		        editor_data, created_by, published_at, archived_at, created_at, updated_at
		 FROM template_versions
		 WHERE template_id = @template_id
		 ORDER BY version_number DESC`,
		pgx.NamedArgs{"template_id": templateID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing template versions: %w", err)
	}

	versions, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.TemplateVersion])
	if err != nil {
		return nil, fmt.Errorf("collecting template versions: %w", err)
	}

	return versions, nil
}

// --- Locales ---

func (r *TemplateRepo) SetLocale(ctx context.Context, locale *domain.TemplateVersionLocale) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO template_version_locales (id, template_version_id, locale, subject, preview_text,
		                                       from_name, body_mjml, editor_data)
		 VALUES (@id, @template_version_id, @locale, @subject, @preview_text,
		         @from_name, @body_mjml, @editor_data)
		 ON CONFLICT ON CONSTRAINT tvl_unique
		 DO UPDATE SET subject = EXCLUDED.subject,
		              preview_text = EXCLUDED.preview_text,
		              from_name = EXCLUDED.from_name,
		              body_mjml = EXCLUDED.body_mjml,
		              editor_data = EXCLUDED.editor_data,
		              updated_at = now()
		 RETURNING id, created_at, updated_at`,
		pgx.NamedArgs{
			"id":                  locale.ID,
			"template_version_id": locale.TemplateVersionID,
			"locale":              locale.Locale,
			"subject":             locale.Subject,
			"preview_text":        locale.PreviewText,
			"from_name":           locale.FromName,
			"body_mjml":           locale.BodyMJML,
			"editor_data":         locale.EditorData,
		},
	)

	if err := row.Scan(&locale.ID, &locale.CreatedAt, &locale.UpdatedAt); err != nil {
		return fmt.Errorf("upserting template version locale: %w", err)
	}

	return nil
}

func (r *TemplateRepo) GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, template_version_id, locale, subject, preview_text, from_name,
		        body_mjml, editor_data, created_at, updated_at
		 FROM template_version_locales
		 WHERE template_version_id = @version_id AND locale = @locale`,
		pgx.NamedArgs{"version_id": versionID, "locale": locale},
	)

	return scanTemplateVersionLocale(row)
}

func (r *TemplateRepo) ListLocales(ctx context.Context, versionID uuid.UUID) ([]*domain.TemplateVersionLocale, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, template_version_id, locale, subject, preview_text, from_name,
		        body_mjml, editor_data, created_at, updated_at
		 FROM template_version_locales
		 WHERE template_version_id = @version_id
		 ORDER BY created_at ASC`,
		pgx.NamedArgs{"version_id": versionID},
	)
	if err != nil {
		return nil, fmt.Errorf("listing template version locales: %w", err)
	}

	locales, err := pgx.CollectRows(rows, scanTemplateVersionLocaleRow)
	if err != nil {
		return nil, fmt.Errorf("collecting template version locales: %w", err)
	}

	return locales, nil
}

func (r *TemplateRepo) DeleteLocale(ctx context.Context, versionID uuid.UUID, locale string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM template_version_locales
		 WHERE template_version_id = @version_id AND locale = @locale`,
		pgx.NamedArgs{"version_id": versionID, "locale": locale},
	)
	if err != nil {
		return fmt.Errorf("deleting template version locale: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("template locale not found")
	}
	return nil
}

// --- Scanners ---

// scanTemplateTypeRow is a pgx.RowToFunc for use with pgx.CollectRows.
func scanTemplateTypeRow(row pgx.CollectableRow) (*domain.TemplateType, error) {
	var tt domain.TemplateType
	err := row.Scan(
		&tt.ID, &tt.Slug, &tt.Name, &tt.Description,
		&tt.WorkspaceID, &tt.AdapterID, &tt.SenderIdentityID, &tt.VariableSchema,
		&tt.CreatedAt, &tt.UpdatedAt, &tt.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning template type row: %w", err)
	}
	return &tt, nil
}

func scanTemplateType(row pgx.Row) (*domain.TemplateType, error) {
	var tt domain.TemplateType
	err := row.Scan(
		&tt.ID, &tt.Slug, &tt.Name, &tt.Description,
		&tt.WorkspaceID, &tt.AdapterID, &tt.SenderIdentityID, &tt.VariableSchema,
		&tt.CreatedAt, &tt.UpdatedAt, &tt.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("template type not found")
		}
		return nil, fmt.Errorf("scanning template type: %w", err)
	}
	return &tt, nil
}

// scanTemplateRow is a pgx.RowToFunc for use with pgx.CollectRows.
func scanTemplateRow(row pgx.CollectableRow) (*domain.Template, error) {
	var t domain.Template
	err := row.Scan(
		&t.ID, &t.TemplateTypeID, &t.WorkspaceID, &t.IsDisabled,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning template row: %w", err)
	}
	return &t, nil
}

func scanTemplate(row pgx.Row) (*domain.Template, error) {
	var t domain.Template
	err := row.Scan(
		&t.ID, &t.TemplateTypeID, &t.WorkspaceID, &t.IsDisabled,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("template not found")
		}
		return nil, fmt.Errorf("scanning template: %w", err)
	}
	return &t, nil
}

func scanTemplateVersion(row pgx.Row) (*domain.TemplateVersion, error) {
	var v domain.TemplateVersion
	err := row.Scan(
		&v.ID, &v.TemplateID, &v.VersionNumber, &v.Status,
		&v.Subject, &v.PreviewText, &v.FromName,
		&v.ReplyTo, &v.BodyMJML, &v.DefaultLocale,
		&v.EditorData, &v.CreatedBy,
		&v.PublishedAt, &v.ArchivedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("template version not found")
		}
		return nil, fmt.Errorf("scanning template version: %w", err)
	}
	return &v, nil
}

func scanTemplateVersionLocale(row pgx.Row) (*domain.TemplateVersionLocale, error) {
	var l domain.TemplateVersionLocale
	err := row.Scan(
		&l.ID, &l.TemplateVersionID, &l.Locale,
		&l.Subject, &l.PreviewText, &l.FromName,
		&l.BodyMJML, &l.EditorData,
		&l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("template version locale not found")
		}
		return nil, fmt.Errorf("scanning template version locale: %w", err)
	}
	return &l, nil
}

// scanTemplateVersionLocaleRow is a pgx.RowToFunc for use with pgx.CollectRows.
func scanTemplateVersionLocaleRow(row pgx.CollectableRow) (*domain.TemplateVersionLocale, error) {
	var l domain.TemplateVersionLocale
	err := row.Scan(
		&l.ID, &l.TemplateVersionID, &l.Locale,
		&l.Subject, &l.PreviewText, &l.FromName,
		&l.BodyMJML, &l.EditorData,
		&l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning template version locale row: %w", err)
	}
	return &l, nil
}
