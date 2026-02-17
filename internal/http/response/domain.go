package response

import (
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// DomainResponse is the JSON response for a domain record.
// SECURITY: DKIMPrivateKeyEncrypted is never exposed.
type DomainResponse struct {
	ID           string           `json:"id"`
	WorkspaceID  *string          `json:"workspace_id,omitempty"`
	DomainName   string           `json:"domain_name"`
	DKIMSelector string           `json:"dkim_selector"`
	DKIMPublicKey string          `json:"dkim_public_key"`
	DNSRecords   []map[string]any `json:"dns_records"`
	Status       string           `json:"status"`
	VerifiedAt   *string          `json:"verified_at,omitempty"`
	LastCheckAt  *string          `json:"last_check_at,omitempty"`
	NextCheckAt  *string          `json:"next_check_at,omitempty"`
	LastError    *string          `json:"last_error,omitempty"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
}

// DomainListResponse is the JSON response for a paginated list of domains.
type DomainListResponse struct {
	Items      []DomainResponse `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// NewDomainResponse maps a domain Domain to a DomainResponse.
func NewDomainResponse(d *domain.Domain) DomainResponse {
	resp := DomainResponse{
		ID:            d.ID.String(),
		DomainName:    d.DomainName,
		DKIMSelector:  d.DKIMSelector,
		DKIMPublicKey: d.DKIMPublicKey,
		DNSRecords:    d.DNSRecords,
		Status:        string(d.Status),
		CreatedAt:     d.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:     d.UpdatedAt.UTC().Format(timeFormat),
	}
	if d.WorkspaceID != nil {
		s := d.WorkspaceID.String()
		resp.WorkspaceID = &s
	}
	if d.VerifiedAt != nil {
		s := d.VerifiedAt.UTC().Format(timeFormat)
		resp.VerifiedAt = &s
	}
	if d.LastCheckAt != nil {
		s := d.LastCheckAt.UTC().Format(timeFormat)
		resp.LastCheckAt = &s
	}
	if d.NextCheckAt != nil {
		s := d.NextCheckAt.UTC().Format(timeFormat)
		resp.NextCheckAt = &s
	}
	if d.LastError != nil {
		resp.LastError = d.LastError
	}
	return resp
}

// NewDomainListResponse maps a PageResult of domains to a DomainListResponse.
func NewDomainListResponse(page *port.PageResult[domain.Domain]) DomainListResponse {
	items := make([]DomainResponse, len(page.Items))
	for i, d := range page.Items {
		items[i] = NewDomainResponse(d)
	}
	return DomainListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}
