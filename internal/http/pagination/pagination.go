package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

const (
	defaultLimit = 25
	maxLimit     = 100
)

// CursorData holds the decoded cursor components for keyset pagination.
type CursorData struct {
	Timestamp time.Time `json:"t"`
	ID        uuid.UUID `json:"id"`
}

// EncodeCursor encodes a timestamp and ID into a base64url-encoded JSON cursor string.
func EncodeCursor(t time.Time, id uuid.UUID) string {
	data, _ := json.Marshal(CursorData{Timestamp: t, ID: id})
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeCursor decodes a base64url-encoded JSON cursor string into its components.
// Returns domain.ErrInvalidCursor on any failure.
func DecodeCursor(cursor string) (*CursorData, error) {
	if cursor == "" {
		return nil, fmt.Errorf("%w: empty cursor", domain.ErrInvalidCursor)
	}

	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid encoding", domain.ErrInvalidCursor)
	}

	var cd CursorData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return nil, fmt.Errorf("%w: invalid format", domain.ErrInvalidCursor)
	}

	return &cd, nil
}

// ParseListOptions extracts cursor and limit query parameters from the request.
// Defaults: limit=25, max=100. Invalid or non-positive limits fall back to default.
func ParseListOptions(c *echo.Context) port.ListOptions {
	cursor := c.QueryParam("cursor")

	limit := defaultLimit
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return port.ListOptions{
		Cursor: cursor,
		Limit:  limit,
	}
}
