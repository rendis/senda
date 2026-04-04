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
	Name                 *string          `json:"name"`
	Config               *json.RawMessage `json:"config"`
	IsDefault            *bool            `json:"is_default"`
	RateLimitPerSecond   *int             `json:"rate_limit_per_second"`
	ConfigurationSetName *string          `json:"configuration_set_name,omitempty"`
}

// TestAdapterRequest is the request body for POST adapters/:id/test.
type TestAdapterRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// CreateManualIdentityRequest is the request body for POST adapters/:id/identities.
type CreateManualIdentityRequest struct {
	Identity    string  `json:"identity"`
	DisplayName *string `json:"display_name,omitempty"`
}

// UpdateWorkspaceAccessRequest replaces the set of tenant workspaces allowed to use a shared resource.
type UpdateWorkspaceAccessRequest struct {
	WorkspaceIDs []string `json:"workspace_ids"`
}
