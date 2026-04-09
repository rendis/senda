package request

// CreateInjectorRequest is the request body for POST injectors.
type CreateInjectorRequest struct {
	Name        string                `json:"name"`
	Description *string               `json:"description"`
	Fields      []CreateInjectorField `json:"fields"`
}

// UpdateInjectorRequest is the request body for PUT injectors/:name.
type UpdateInjectorRequest = CreateInjectorRequest

// CreateInjectorField represents a single field within a CreateInjectorRequest.
type CreateInjectorField struct {
	FieldName      string  `json:"field_name"`
	FieldType      string  `json:"field_type"`
	Description    *string `json:"description"`
	Position       int     `json:"position"`
	DefaultValue   any     `json:"default_value,omitempty"`
	AllowOverwrite *bool   `json:"allow_overwrite,omitempty"`
}

// SetInjectorValuesRequest is the request body for PUT injectors/:name/values.
type SetInjectorValuesRequest struct {
	Values []InjectorFieldValue `json:"values"`
}

// InjectorFieldValue maps a field name to its value.
type InjectorFieldValue struct {
	FieldName string `json:"field_name"`
	Value     string `json:"value"`
}

// UpdateInjectorFieldRequest updates one field's default resolution settings.
type UpdateInjectorFieldRequest struct {
	DefaultValue   any   `json:"default_value,omitempty"`
	AllowOverwrite *bool `json:"allow_overwrite,omitempty"`
}
