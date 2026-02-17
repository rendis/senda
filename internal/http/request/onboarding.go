package request

// OnboardingSetupRequest is the request body for POST /api/v1/onboarding/setup.
type OnboardingSetupRequest struct {
	TenantCode string `json:"tenant_code"`
	TenantName string `json:"tenant_name"`
}
