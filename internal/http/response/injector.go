package response

import (
	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// InjectorDefinitionResponse is the JSON response for an injector definition.
type InjectorDefinitionResponse struct {
	ID                  string  `json:"id"`
	WorkspaceID         *string `json:"workspace_id,omitempty"`
	Name                string  `json:"name"`
	Description         *string `json:"description,omitempty"`
	OwnerScope          string  `json:"owner_scope,omitempty"`
	InheritedFromSystem bool    `json:"inherited_from_system"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

// InjectorFieldResponse is the JSON response for an injector field.
type InjectorFieldResponse struct {
	ID             string  `json:"id"`
	FieldName      string  `json:"field_name"`
	FieldType      string  `json:"field_type"`
	Description    *string `json:"description,omitempty"`
	Position       int     `json:"position"`
	DefaultValue   any     `json:"default_value,omitempty"`
	AllowOverwrite bool    `json:"allow_overwrite"`
}

// InjectorValueResponse is the JSON response for an injector field value.
type InjectorValueResponse struct {
	ID          string  `json:"id"`
	FieldName   string  `json:"field_name"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	Value       string  `json:"value"`
	UpdatedAt   string  `json:"updated_at"`
}

// InjectorDetailResponse combines definition, fields, and values.
type InjectorDetailResponse struct {
	InjectorDefinitionResponse
	Fields []InjectorFieldResponse `json:"fields"`
	Values []InjectorValueResponse `json:"values,omitempty"`
}

// InjectorListResponse is the JSON response for a list of injector definitions.
type InjectorListResponse struct {
	Items []InjectorDetailResponse `json:"items"`
}

// NewInjectorDefinitionResponse maps a domain InjectorDefinition to a response.
func NewInjectorDefinitionResponse(d *domain.InjectorDefinition) InjectorDefinitionResponse {
	resp := InjectorDefinitionResponse{
		ID:                  d.ID.String(),
		Name:                d.Name,
		Description:         d.Description,
		OwnerScope:          d.OwnerScope,
		InheritedFromSystem: d.InheritedFromSystem,
		CreatedAt:           d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           d.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if d.WorkspaceID != nil {
		s := d.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	return resp
}

// NewInjectorFieldResponse maps a domain InjectorField to a response.
func NewInjectorFieldResponse(f *domain.InjectorField) InjectorFieldResponse {
	return InjectorFieldResponse{
		ID:             f.ID.String(),
		FieldName:      f.FieldName,
		FieldType:      string(f.FieldType),
		Description:    f.Description,
		Position:       f.Position,
		DefaultValue:   f.DefaultValue,
		AllowOverwrite: f.AllowOverwrite,
	}
}

// NewInjectorValueResponse maps a domain InjectorValue to a response.
func NewInjectorValueResponse(v *domain.InjectorValue) InjectorValueResponse {
	resp := InjectorValueResponse{
		ID:        v.ID.String(),
		FieldName: v.FieldName,
		Value:     v.Value,
		UpdatedAt: v.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if v.WorkspaceID != nil {
		s := v.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	return resp
}

// NewInjectorListResponse maps a slice of domain InjectorDefinition to a list response.
func NewInjectorListResponse(defs []*domain.InjectorDefinition, fieldsByDefinition map[uuid.UUID][]*domain.InjectorField) InjectorListResponse {
	items := make([]InjectorDetailResponse, len(defs))
	for i, d := range defs {
		items[i] = NewInjectorCreateResponse(d, fieldsByDefinition[d.ID])
	}
	return InjectorListResponse{Items: items}
}

// NewInjectorDetailResponse builds a detail response from definition, fields, and values.
func NewInjectorDetailResponse(def *domain.InjectorDefinition, fields []*domain.InjectorField, values []*domain.InjectorValue) InjectorDetailResponse {
	fieldResps := make([]InjectorFieldResponse, len(fields))
	for i, f := range fields {
		fieldResps[i] = NewInjectorFieldResponse(f)
	}
	valueResps := make([]InjectorValueResponse, len(values))
	for i, v := range values {
		valueResps[i] = NewInjectorValueResponse(v)
	}
	return InjectorDetailResponse{
		InjectorDefinitionResponse: NewInjectorDefinitionResponse(def),
		Fields:                     fieldResps,
		Values:                     valueResps,
	}
}

// NewInjectorCreateResponse builds a detail response from definition and fields (no values yet).
func NewInjectorCreateResponse(def *domain.InjectorDefinition, fields []*domain.InjectorField) InjectorDetailResponse {
	fieldResps := make([]InjectorFieldResponse, len(fields))
	for i, f := range fields {
		fieldResps[i] = NewInjectorFieldResponse(f)
	}
	return InjectorDetailResponse{
		InjectorDefinitionResponse: NewInjectorDefinitionResponse(def),
		Fields:                     fieldResps,
	}
}

// Ensure PageResult is usable (compile-time check).
var _ = (*port.PageResult[domain.InjectorDefinition])(nil)
