package response

// OnboardingStatusResponse is the JSON response for GET /api/v1/onboarding/status.
type OnboardingStatusResponse struct {
	NeedsOnboarding bool `json:"needs_onboarding"`
}

// OnboardingSetupResponse is the JSON response for POST /api/v1/onboarding/setup.
type OnboardingSetupResponse struct {
	Member    OnboardingMemberSummary    `json:"member"`
	Tenant    OnboardingTenantSummary    `json:"tenant"`
	Workspace OnboardingWorkspaceSummary `json:"workspace"`
}

// OnboardingMemberSummary is a minimal member representation for onboarding.
type OnboardingMemberSummary struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// OnboardingTenantSummary is a minimal tenant representation for onboarding.
type OnboardingTenantSummary struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// OnboardingWorkspaceSummary is a minimal workspace representation for onboarding.
type OnboardingWorkspaceSummary struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}
