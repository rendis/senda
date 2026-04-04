package response

import (
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// DashboardStatsResponse is the JSON response for the dashboard stats endpoint.
type DashboardStatsResponse struct {
	Totals         DashboardTotalsResp          `json:"totals"`
	Rates          DashboardRatesResp           `json:"rates"`
	TimeSeries     []DashboardTimePointResp     `json:"time_series"`
	RecentEmails   []DashboardRecentEmailResp   `json:"recent_emails"`
	RecentActivity []DashboardActivityResp      `json:"recent_activity"`
	ByAdapter      []DashboardAdapterTotalsResp `json:"by_adapter"`
}

// DashboardAdapterTotalsResp holds per-adapter email totals.
type DashboardAdapterTotalsResp struct {
	AdapterID        string              `json:"adapter_id"`
	AdapterName      string              `json:"adapter_name"`
	AdapterType      string              `json:"adapter_type"`
	SenderIdentityID *string             `json:"sender_identity_id,omitempty"`
	FromEmail        string              `json:"from_email"`
	Totals           DashboardTotalsResp `json:"totals"`
}

// DashboardTotalsResp holds aggregated email counts.
type DashboardTotalsResp struct {
	Sent       int64 `json:"sent"`
	Delivered  int64 `json:"delivered"`
	Bounced    int64 `json:"bounced"`
	Complained int64 `json:"complained"`
	Failed     int64 `json:"failed"`
}

// DashboardRatesResp holds computed delivery/bounce/complaint rates.
type DashboardRatesResp struct {
	DeliveryRate  float64 `json:"delivery_rate"`
	BounceRate    float64 `json:"bounce_rate"`
	ComplaintRate float64 `json:"complaint_rate"`
}

// DashboardTimePointResp holds a single day's email counts.
type DashboardTimePointResp struct {
	Date      string `json:"date"`
	Sent      int64  `json:"sent"`
	Delivered int64  `json:"delivered"`
	Bounced   int64  `json:"bounced"`
	Failed    int64  `json:"failed"`
}

// DashboardRecentEmailResp holds a summary of a recent email.
type DashboardRecentEmailResp struct {
	ID               string `json:"id"`
	TrackingID       string `json:"tracking_id"`
	RecipientEmail   string `json:"recipient_email"`
	TemplateTypeSlug string `json:"template_type_slug"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
}

// DashboardActivityResp holds a summary of a recent audit log entry.
type DashboardActivityResp struct {
	ID         string `json:"id"`
	ActorEmail string `json:"actor_email"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	CreatedAt  string `json:"created_at"`
}

// NewDashboardStatsResponse builds the complete dashboard response from domain/port types.
func NewDashboardStatsResponse(
	totals *port.DashboardTotals,
	series []port.DashboardTimePoint,
	recentEmails []port.DashboardRecentEmail,
	auditLogs []*domain.AuditLog,
	byAdapter []port.DashboardAdapterTotals,
) DashboardStatsResponse {
	// Compute rates with zero-division guard.
	var rates DashboardRatesResp
	if totals.Sent > 0 {
		rates.DeliveryRate = float64(totals.Delivered) / float64(totals.Sent)
		rates.BounceRate = float64(totals.Bounced) / float64(totals.Sent)
		rates.ComplaintRate = float64(totals.Complained) / float64(totals.Sent)
	}

	// Map time series.
	tsResp := make([]DashboardTimePointResp, len(series))
	for i, pt := range series {
		tsResp[i] = DashboardTimePointResp{
			Date:      pt.Date.Format("2006-01-02"),
			Sent:      pt.Sent,
			Delivered: pt.Delivered,
			Bounced:   pt.Bounced,
			Failed:    pt.Failed,
		}
	}

	// Map recent emails.
	emailResp := make([]DashboardRecentEmailResp, len(recentEmails))
	for i, e := range recentEmails {
		emailResp[i] = DashboardRecentEmailResp{
			ID:               e.ID.String(),
			TrackingID:       e.TrackingID,
			RecipientEmail:   e.RecipientEmail,
			TemplateTypeSlug: e.TemplateTypeSlug,
			Status:           string(e.Status),
			CreatedAt:        formatTime(e.CreatedAt),
		}
	}

	// Map recent activity.
	actResp := make([]DashboardActivityResp, len(auditLogs))
	for i, a := range auditLogs {
		actResp[i] = DashboardActivityResp{
			ID:         a.ID.String(),
			ActorEmail: a.ActorEmail,
			Action:     string(a.Action),
			EntityType: a.EntityType,
			CreatedAt:  formatTime(a.CreatedAt),
		}
	}

	// Map by-adapter breakdown.
	adapterResp := make([]DashboardAdapterTotalsResp, len(byAdapter))
	for i, at := range byAdapter {
		adapterResp[i] = DashboardAdapterTotalsResp{
			AdapterID:   at.AdapterID.String(),
			AdapterName: at.AdapterName,
			AdapterType: at.AdapterType,
			FromEmail:   at.FromEmail,
			Totals: DashboardTotalsResp{
				Sent:       at.Totals.Sent,
				Delivered:  at.Totals.Delivered,
				Bounced:    at.Totals.Bounced,
				Complained: at.Totals.Complained,
				Failed:     at.Totals.Failed,
			},
		}
		if at.SenderIdentityID != nil {
			id := at.SenderIdentityID.String()
			adapterResp[i].SenderIdentityID = &id
		}
	}

	return DashboardStatsResponse{
		Totals: DashboardTotalsResp{
			Sent:       totals.Sent,
			Delivered:  totals.Delivered,
			Bounced:    totals.Bounced,
			Complained: totals.Complained,
			Failed:     totals.Failed,
		},
		Rates:          rates,
		TimeSeries:     tsResp,
		RecentEmails:   emailResp,
		RecentActivity: actResp,
		ByAdapter:      adapterResp,
	}
}
