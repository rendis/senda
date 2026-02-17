package request

// CreateTenantRequest is the request body for POST /api/v1/manage/tenants.
type CreateTenantRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// UpdateTenantRequest is the request body for PUT /api/v1/manage/tenants/:tenant_code.
type UpdateTenantRequest struct {
	Name *string `json:"name"`
}
