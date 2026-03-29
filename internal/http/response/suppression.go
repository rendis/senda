package response

import "github.com/rendis/senda/internal/domain"

// SuppressionWorkspaceResponse is the JSON response for a workspace suppression entry.
type SuppressionWorkspaceResponse struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	Email         string  `json:"email"`
	Reason        string  `json:"reason"`
	SourceEmailID *string `json:"source_email_id,omitempty"`
	Notes         *string `json:"notes,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// NewSuppressionWorkspaceResponse maps a domain SuppressionWorkspace to a response.
func NewSuppressionWorkspaceResponse(s *domain.SuppressionWorkspace) SuppressionWorkspaceResponse {
	resp := SuppressionWorkspaceResponse{
		ID:          s.ID.String(),
		WorkspaceID: s.WorkspaceID.String(),
		Email:       s.Email,
		Reason:      string(s.Reason),
		Notes:       s.Notes,
		CreatedAt:   formatTime(s.CreatedAt),
	}
	if s.SourceEmailID != nil {
		str := s.SourceEmailID.String()
		resp.SourceEmailID = &str
	}
	return resp
}

// SuppressionCheckResponse is the JSON response for checking suppression status.
type SuppressionCheckResponse struct {
	Email      string `json:"email"`
	Suppressed bool   `json:"suppressed"`
	Reason     string `json:"reason,omitempty"`
}
