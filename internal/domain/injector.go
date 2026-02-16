package domain

import (
	"time"

	"github.com/google/uuid"
)

type InjectorFieldType string

const (
	FieldTypeText   InjectorFieldType = "text"
	FieldTypeNumber InjectorFieldType = "number"
	FieldTypeBool   InjectorFieldType = "bool"
	FieldTypeImg    InjectorFieldType = "img"
	FieldTypeURL    InjectorFieldType = "url"
	FieldTypeHTML   InjectorFieldType = "html"
)

type InjectorDefinition struct {
	ID          uuid.UUID
	WorkspaceID *uuid.UUID // nil = global
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type InjectorField struct {
	ID           uuid.UUID
	DefinitionID uuid.UUID
	FieldName    string
	FieldType    InjectorFieldType
	IsRequired   bool
	DefaultValue *string
	Description  *string
	SortOrder    int
}

type InjectorValue struct {
	ID          uuid.UUID
	FieldID     uuid.UUID
	WorkspaceID *uuid.UUID // scope where value is set
	Value       string     // stored as text, cast by field type
	UpdatedAt   time.Time
}
