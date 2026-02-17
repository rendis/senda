package request

// RegisterDomainRequest is the request body for POST domains.
type RegisterDomainRequest struct {
	DomainName string `json:"domain_name"`
}
