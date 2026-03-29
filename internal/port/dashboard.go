package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
)

// DashboardStatsParams defines the scope and time range for dashboard queries.
type DashboardStatsParams struct {
	TenantID    *uuid.UUID
	WorkspaceID *uuid.UUID
	Since       time.Time
	Until       time.Time
}

// DashboardTotals holds aggregated email counts by status.
type DashboardTotals struct {
	Sent       int64
	Delivered  int64
	Bounced    int64
	Complained int64
	Failed     int64
}

// DashboardTimePoint holds daily aggregated email counts.
type DashboardTimePoint struct {
	Date      time.Time
	Sent      int64
	Delivered int64
	Bounced   int64
	Failed    int64
}

// DashboardRecentEmail holds a summary of a recent email for the dashboard.
type DashboardRecentEmail struct {
	ID               uuid.UUID
	TrackingID       string
	RecipientEmail   string
	TemplateTypeSlug string
	Status           domain.EmailStatus
	CreatedAt        time.Time
}

// DashboardAdapterTotals holds aggregated email counts for a single adapter.
type DashboardAdapterTotals struct {
	AdapterID   uuid.UUID
	AdapterName string
	AdapterType string
	Totals      DashboardTotals
}

// DashboardStore provides read-only access to email metrics for dashboard views.
type DashboardStore interface {
	GetTotals(ctx context.Context, p DashboardStatsParams) (*DashboardTotals, error)
	GetTimeSeries(ctx context.Context, p DashboardStatsParams) ([]DashboardTimePoint, error)
	GetRecentEmails(ctx context.Context, p DashboardStatsParams, limit int) ([]DashboardRecentEmail, error)
	GetTotalsByAdapter(ctx context.Context, p DashboardStatsParams) ([]DashboardAdapterTotals, error)
}
