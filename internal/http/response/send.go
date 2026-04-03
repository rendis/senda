package response

import "github.com/rendis/senda/internal/service"

// SendEmailResponse is the JSON response for POST /api/v1/send.
type SendEmailResponse struct {
	Status           string                  `json:"status"`
	TrackingIDs      []TrackingEntryResponse `json:"tracking_ids"`
	ExternalID       *string                 `json:"external_id,omitempty"`
	TemplateResolved string                  `json:"template_resolved"`
	TemplateVersion  int                     `json:"template_version"`
}

// SendBatchResponse is the JSON response for POST /api/v1/send/batch.
type SendBatchResponse struct {
	Status           string                  `json:"status"`
	TemplateResolved string                  `json:"template_resolved"`
	Items            []SendBatchItemResponse `json:"items"`
	AcceptedCount    int                     `json:"accepted_count"`
	SuppressedCount  int                     `json:"suppressed_count"`
	FailedCount      int                     `json:"failed_count"`
}

// TrackingEntryResponse maps a recipient to their tracking ID and per-recipient status.
type TrackingEntryResponse struct {
	To         string `json:"to"`
	TrackingID string `json:"tracking_id"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// SendBatchItemResponse maps one batch item to its result.
type SendBatchItemResponse struct {
	Index      int     `json:"index"`
	To         string  `json:"to"`
	TrackingID string  `json:"tracking_id,omitempty"`
	Status     string  `json:"status"`
	ExternalID *string `json:"external_id,omitempty"`
	Error      string  `json:"error,omitempty"`
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

// NewSendBatchResponse maps a service.SendBatchResponse to the HTTP response.
func NewSendBatchResponse(r *service.SendBatchResponse) SendBatchResponse {
	items := make([]SendBatchItemResponse, len(r.Items))
	for i, item := range r.Items {
		items[i] = SendBatchItemResponse{
			Index:      item.Index,
			To:         item.To,
			TrackingID: item.TrackingID,
			Status:     item.Status,
			ExternalID: item.ExternalID,
			Error:      item.Error,
		}
	}

	return SendBatchResponse{
		Status:           r.Status,
		TemplateResolved: r.TemplateResolved,
		Items:            items,
		AcceptedCount:    r.AcceptedCount,
		SuppressedCount:  r.SuppressedCount,
		FailedCount:      r.FailedCount,
	}
}
