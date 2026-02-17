package request

import "encoding/json"

// CreateAdapterRequest is the request body for POST adapters.
type CreateAdapterRequest struct {
	Name               string          `json:"name"`
	AdapterType        string          `json:"adapter_type"`
	Config             json.RawMessage `json:"config"`
	IsDefault          bool            `json:"is_default"`
	RateLimitPerSecond int             `json:"rate_limit_per_second"`
}

// UpdateAdapterRequest is the request body for PUT adapters/:id.
type UpdateAdapterRequest struct {
	Name               *string          `json:"name"`
	Config             *json.RawMessage `json:"config"`
	IsDefault          *bool            `json:"is_default"`
	RateLimitPerSecond *int             `json:"rate_limit_per_second"`
}
