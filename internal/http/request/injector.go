package request

// CreateInjectorRequest is the request body for POST injectors.
type CreateInjectorRequest struct {
	Name        string                `json:"name"`
	Description *string               `json:"description"`
	Fields      []CreateInjectorField `json:"fields"`
}

// CreateInjectorField represents a single field within a CreateInjectorRequest.
type CreateInjectorField struct {
	FieldName   string  `json:"field_name"`
	FieldType   string  `json:"field_type"`
	Description *string `json:"description"`
	Position    int     `json:"position"`
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
