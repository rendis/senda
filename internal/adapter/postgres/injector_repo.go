package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/pkg/apperr"
)

// InjectorRepo implements port.InjectorStore using PostgreSQL.
type InjectorRepo struct {
	pool *pgxpool.Pool
}

// NewInjectorRepo creates a new InjectorRepo.
func NewInjectorRepo(pool *pgxpool.Pool) *InjectorRepo {
	return &InjectorRepo{pool: pool}
}

func (r *InjectorRepo) CreateDefinition(ctx context.Context, def *domain.InjectorDefinition) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO injector_definitions (id, name, workspace_id, description)
		 VALUES (@id, @name, @workspace_id, @description)
		 RETURNING created_at, updated_at`,
		pgx.NamedArgs{
			"id":           def.ID,
			"name":         def.Name,
			"workspace_id": def.WorkspaceID,
			"description":  def.Description,
		},
	)

	if err := row.Scan(&def.CreatedAt, &def.UpdatedAt); err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting injector definition: %w", err)
	}

	return nil
}

func (r *InjectorRepo) GetDefinitionByID(ctx context.Context, id uuid.UUID) (*domain.InjectorDefinition, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, workspace_id, description, created_at, updated_at, deleted_at
		 FROM injector_definitions
		 WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)

	return scanInjectorDefinition(row)
}

func (r *InjectorRepo) SoftDeleteDefinition(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE injector_definitions SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL`,
		pgx.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("soft-deleting injector: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("injector %s not found", id)
	}
	return nil
}

func (r *InjectorRepo) FindDefinitionByName(ctx context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error) {
	var row pgx.Row
	if workspaceID == nil {
		row = r.pool.QueryRow(ctx,
			`SELECT id, name, workspace_id, description, created_at, updated_at, deleted_at
			 FROM injector_definitions
			 WHERE name = @name AND workspace_id IS NULL AND deleted_at IS NULL`,
			pgx.NamedArgs{"name": name},
		)
	} else {
		row = r.pool.QueryRow(ctx,
			`SELECT id, name, workspace_id, description, created_at, updated_at, deleted_at
			 FROM injector_definitions
			 WHERE name = @name AND workspace_id = @workspace_id AND deleted_at IS NULL`,
			pgx.NamedArgs{"name": name, "workspace_id": *workspaceID},
		)
	}

	return scanInjectorDefinition(row)
}

func (r *InjectorRepo) ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
	scopes, includeGlobal := splitChain(chain)

	rows, err := r.pool.Query(ctx,
		`SELECT id, name, workspace_id, description, created_at, updated_at, deleted_at
		 FROM injector_definitions
		 WHERE (workspace_id = ANY(@scopes) OR (@include_global::bool AND workspace_id IS NULL))
		   AND deleted_at IS NULL
		 ORDER BY id DESC`,
		pgx.NamedArgs{
			"scopes":         scopes,
			"include_global": includeGlobal,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("listing injector definitions in chain: %w", err)
	}

	defs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.InjectorDefinition])
	if err != nil {
		return nil, fmt.Errorf("collecting injector definitions: %w", err)
	}

	return defs, nil
}

func (r *InjectorRepo) CreateField(ctx context.Context, field *domain.InjectorField) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO injector_fields (id, injector_definition_id, field_name, field_type, description, position)
		 VALUES (@id, @injector_definition_id, @field_name, @field_type, @description, @position)`,
		pgx.NamedArgs{
			"id":                      field.ID,
			"injector_definition_id":  field.InjectorDefinitionID,
			"field_name":              field.FieldName,
			"field_type":              field.FieldType,
			"description":             field.Description,
			"position":                field.Position,
		},
	)
	if err != nil {
		if appErr := classifyPgError(err); appErr != nil {
			return appErr
		}
		return fmt.Errorf("inserting injector field: %w", err)
	}

	return nil
}

func (r *InjectorRepo) GetFieldsByDefinition(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, injector_definition_id, field_name, field_type, description, position
		 FROM injector_fields
		 WHERE injector_definition_id = @def_id
		 ORDER BY position`,
		pgx.NamedArgs{"def_id": defID},
	)
	if err != nil {
		return nil, fmt.Errorf("querying injector fields: %w", err)
	}

	fields, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.InjectorField])
	if err != nil {
		return nil, fmt.Errorf("collecting injector fields: %w", err)
	}

	return fields, nil
}

func (r *InjectorRepo) SetValue(ctx context.Context, val *domain.InjectorValue) error {
	jsonVal, err := json.Marshal(val.Value)
	if err != nil {
		return fmt.Errorf("marshaling injector value: %w", err)
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO injector_values (id, injector_definition_id, field_name, workspace_id, value)
		 VALUES (@id, @injector_definition_id, @field_name, @workspace_id, @value)
		 ON CONFLICT ON CONSTRAINT injector_value_unique
		 DO UPDATE SET value = EXCLUDED.value, updated_at = now()
		 RETURNING id, updated_at`,
		pgx.NamedArgs{
			"id":                      val.ID,
			"injector_definition_id":  val.InjectorDefinitionID,
			"field_name":              val.FieldName,
			"workspace_id":            val.WorkspaceID,
			"value":                   jsonVal,
		},
	)

	if err := row.Scan(&val.ID, &val.UpdatedAt); err != nil {
		return fmt.Errorf("upserting injector value: %w", err)
	}

	return nil
}

func (r *InjectorRepo) GetValues(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error) {
	scopes, includeGlobal := splitChain(chain)

	rows, err := r.pool.Query(ctx,
		`SELECT id, injector_definition_id, field_name, workspace_id, value, updated_at
		 FROM injector_values
		 WHERE injector_definition_id = @def_id
		   AND (workspace_id = ANY(@scopes) OR (@include_global::bool AND workspace_id IS NULL))`,
		pgx.NamedArgs{
			"def_id":         defID,
			"scopes":         scopes,
			"include_global": includeGlobal,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("querying injector values: %w", err)
	}

	vals, err := pgx.CollectRows(rows, scanInjectorValue)
	if err != nil {
		return nil, fmt.Errorf("collecting injector values: %w", err)
	}

	return vals, nil
}

func (r *InjectorRepo) GetAllFieldsByDefinitions(ctx context.Context, defIDs []uuid.UUID) (map[uuid.UUID][]*domain.InjectorField, error) {
	if len(defIDs) == 0 {
		return make(map[uuid.UUID][]*domain.InjectorField), nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, injector_definition_id, field_name, field_type, description, position
		 FROM injector_fields
		 WHERE injector_definition_id = ANY(@def_ids)
		 ORDER BY injector_definition_id, position`,
		pgx.NamedArgs{"def_ids": defIDs},
	)
	if err != nil {
		return nil, fmt.Errorf("batch querying injector fields: %w", err)
	}

	fields, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[domain.InjectorField])
	if err != nil {
		return nil, fmt.Errorf("collecting batch injector fields: %w", err)
	}

	result := make(map[uuid.UUID][]*domain.InjectorField, len(defIDs))
	for _, f := range fields {
		result[f.InjectorDefinitionID] = append(result[f.InjectorDefinitionID], f)
	}
	return result, nil
}

func (r *InjectorRepo) GetAllValuesByDefinitions(ctx context.Context, defIDs []uuid.UUID, chain []uuid.NullUUID) (map[uuid.UUID][]*domain.InjectorValue, error) {
	if len(defIDs) == 0 {
		return make(map[uuid.UUID][]*domain.InjectorValue), nil
	}

	scopes, includeGlobal := splitChain(chain)

	rows, err := r.pool.Query(ctx,
		`SELECT id, injector_definition_id, field_name, workspace_id, value, updated_at
		 FROM injector_values
		 WHERE injector_definition_id = ANY(@def_ids)
		   AND (workspace_id = ANY(@scopes) OR (@include_global::bool AND workspace_id IS NULL))`,
		pgx.NamedArgs{
			"def_ids":        defIDs,
			"scopes":         scopes,
			"include_global": includeGlobal,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("batch querying injector values: %w", err)
	}

	vals, err := pgx.CollectRows(rows, scanInjectorValue)
	if err != nil {
		return nil, fmt.Errorf("collecting batch injector values: %w", err)
	}

	result := make(map[uuid.UUID][]*domain.InjectorValue, len(defIDs))
	for _, v := range vals {
		result[v.InjectorDefinitionID] = append(result[v.InjectorDefinitionID], v)
	}
	return result, nil
}

// scanInjectorDefinition scans a single InjectorDefinition from a row.
func scanInjectorDefinition(row pgx.Row) (*domain.InjectorDefinition, error) {
	var d domain.InjectorDefinition
	err := row.Scan(&d.ID, &d.Name, &d.WorkspaceID, &d.Description, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("injector definition not found")
		}
		return nil, fmt.Errorf("scanning injector definition: %w", err)
	}
	return &d, nil
}

// scanInjectorValue scans a single InjectorValue, unwrapping the JSONB value column to a string.
func scanInjectorValue(row pgx.CollectableRow) (*domain.InjectorValue, error) {
	var v domain.InjectorValue
	var jsonVal []byte
	err := row.Scan(&v.ID, &v.InjectorDefinitionID, &v.FieldName, &v.WorkspaceID, &jsonVal, &v.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning injector value: %w", err)
	}
	if err := json.Unmarshal(jsonVal, &v.Value); err != nil {
		return nil, fmt.Errorf("unmarshaling injector value: %w", err)
	}
	return &v, nil
}

// splitChain separates a chain of nullable UUIDs into a slice of non-null UUIDs
// and a boolean indicating whether the chain includes a NULL (global) scope.
func splitChain(chain []uuid.NullUUID) (scopes []uuid.UUID, includeGlobal bool) {
	for _, entry := range chain {
		if !entry.Valid {
			includeGlobal = true
		} else {
			scopes = append(scopes, entry.UUID)
		}
	}
	return scopes, includeGlobal
}
