package request

// SendEmailRequest is the request body for POST /api/v1/send.
type SendEmailRequest struct {
	Ref        string         `json:"ref"`
	To         []string       `json:"to"`
	CC         []string       `json:"cc,omitempty"`
	BCC        []string       `json:"bcc,omitempty"`
	Variables  map[string]any `json:"variables,omitempty"`
	ExternalID *string        `json:"external_id,omitempty"`
	Locale     *string        `json:"locale,omitempty"`
}

// AddSuppressionRequest is the request body for POST suppression.
type AddSuppressionRequest struct {
	Email  string  `json:"email"`
	Reason string  `json:"reason,omitempty"`
	Notes  *string `json:"notes,omitempty"`
}
