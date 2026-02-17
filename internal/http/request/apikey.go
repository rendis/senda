package request

// CreateAPIKeyRequest is the request body for POST api-keys.
type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}
