package request

// UpdatePreferencesRequest is the body for POST /api/v1/u/:token/preferences.
type UpdatePreferencesRequest struct {
	Changes []PreferenceChange `json:"changes"`
}

// PreferenceChange is a single subscription toggle from the preference center.
type PreferenceChange struct {
	TemplateTypeSlug string `json:"template_type_slug"`
	Subscribed       bool   `json:"subscribed"`
}
