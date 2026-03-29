package handler

import (
	"encoding/csv"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// DataPlaneEmailHandler handles API-key-scoped email query endpoints.
type DataPlaneEmailHandler struct {
	store port.EmailStore
}

// NewDataPlaneEmailHandler creates a new DataPlaneEmailHandler.
func NewDataPlaneEmailHandler(store port.EmailStore) *DataPlaneEmailHandler {
	return &DataPlaneEmailHandler{store: store}
}

func workspaceIDFromContext(c *echo.Context) (uuid.UUID, error) {
	wsID, ok := c.Get(middleware.ContextKeyWorkspaceID).(uuid.UUID)
	if !ok || wsID == uuid.Nil {
		return uuid.Nil, response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "workspace context required (API key auth)")
	}
	return wsID, nil
}

// List handles GET /api/v1/emails.
func (h *DataPlaneEmailHandler) List(c *echo.Context) error {
	wsID, err := workspaceIDFromContext(c)
	if err != nil {
		return err
	}

	cursor := c.QueryParam("cursor")
	limit := parseLimit(c)
	filters := parseEmailFilters(c)

	emails, nextCursor, err := h.store.QueryByWorkspace(c.Request().Context(), wsID, filters, cursor, limit)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewEmailListResponse(emails, nextCursor))
}

// GetByTrackingID handles GET /api/v1/emails/:tracking_id.
func (h *DataPlaneEmailHandler) GetByTrackingID(c *echo.Context) error {
	wsID, err := workspaceIDFromContext(c)
	if err != nil {
		return err
	}

	trackingID := c.Param("tracking_id")
	if trackingID == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "tracking_id is required")
	}

	ctx := c.Request().Context()
	email, err := h.store.GetByTrackingID(ctx, trackingID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if email.WorkspaceID != wsID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	events, err := h.store.GetEvents(ctx, email.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewEmailDetailResponse(email, events))
}

// GetEvents handles GET /api/v1/emails/:tracking_id/events.
func (h *DataPlaneEmailHandler) GetEvents(c *echo.Context) error {
	wsID, err := workspaceIDFromContext(c)
	if err != nil {
		return err
	}

	trackingID := c.Param("tracking_id")
	if trackingID == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "tracking_id is required")
	}

	ctx := c.Request().Context()
	email, err := h.store.GetByTrackingID(ctx, trackingID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if email.WorkspaceID != wsID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	events, err := h.store.GetEvents(ctx, email.ID)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewEmailEventListResponse(events))
}

// Export handles GET /api/v1/emails/export as CSV stream.
func (h *DataPlaneEmailHandler) Export(c *echo.Context) error {
	wsID, err := workspaceIDFromContext(c)
	if err != nil {
		return err
	}

	filters := parseEmailFilters(c)
	limit := parseLimit(c)
	if limit < 1 {
		limit = 100
	}
	cursor := c.QueryParam("cursor")

	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="emails_export.csv"`)

	pr, pw := io.Pipe()
	ctx := c.Request().Context()

	go func() {
		w := csv.NewWriter(pw)

		if err := w.Write([]string{
			"tracking_id",
			"external_id",
			"recipient_email",
			"status",
			"template_ref",
			"from_email",
			"subject_rendered",
			"created_at",
			"reply_to",
		}); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		currentCursor := cursor
		for {
			emails, nextCursor, err := h.store.QueryByWorkspace(ctx, wsID, filters, currentCursor, limit)
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}

			for _, email := range emails {
				externalID := ""
				if email.ExternalID != nil {
					externalID = *email.ExternalID
				}

				replyTo := ""
				if email.ReplyTo != nil {
					replyTo = *email.ReplyTo
				}

				if err := w.Write([]string{
					email.TrackingID,
					externalID,
					email.RecipientEmail,
					string(email.Status),
					email.TemplateRef,
					email.FromEmail,
					email.SubjectRendered,
					email.CreatedAt.UTC().Format(time.RFC3339),
					replyTo,
				}); err != nil {
					_ = pw.CloseWithError(err)
					return
				}
			}

			w.Flush()
			if err := w.Error(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}

			if nextCursor == "" {
				break
			}
			currentCursor = nextCursor
		}

		w.Flush()
		if err := w.Error(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		_ = pw.Close()
	}()

	return c.Stream(http.StatusOK, "text/csv; charset=utf-8", pr)
}
