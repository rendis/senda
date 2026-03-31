package response

import "github.com/rendis/senda/internal/service"

// SendEmailResponse is the JSON response for POST /api/v1/send.
type SendEmailResponse struct {
	Status           string                 `json:"status"`
	TrackingIDs      []TrackingEntryResponse `json:"tracking_ids"`
	ExternalID       *string                `json:"external_id,omitempty"`
	TemplateResolved string                 `json:"template_resolved"`
	TemplateVersion  int                    `json:"template_version"`
}

// TrackingEntryResponse maps a recipient to their tracking ID and per-recipient status.
type TrackingEntryResponse struct {
	To         string `json:"to"`
	TrackingID string `json:"tracking_id"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// NewSendEmailResponse maps a service.SendResponse to the HTTP response.
func NewSendEmailResponse(r *service.SendResponse) SendEmailResponse {
	entries := make([]TrackingEntryResponse, len(r.TrackingIDs))
	for i, e := range r.TrackingIDs {
		entries[i] = TrackingEntryResponse{
			To:         e.To,
			TrackingID: e.TrackingID,
			Status:     e.Status,
			Error:      e.Error,
		}
	}
	return SendEmailResponse{
		Status:           r.Status,
		TrackingIDs:      entries,
		ExternalID:       r.ExternalID,
		TemplateResolved: r.TemplateResolved,
		TemplateVersion:  r.TemplateVersion,
	}
}
