package postgres

import (
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/port"
)

const (
	defaultLimit = 25
	maxLimit     = 100
)

// EncodeCursor encodes a UUID as a base64 cursor string.
func EncodeCursor(id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(id[:])
}

// DecodeCursor decodes a base64 cursor string back to a UUID.
func DecodeCursor(cursor string) (uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid cursor: %w", err)
	}
	if len(b) != 16 {
		return uuid.Nil, fmt.Errorf("invalid cursor: expected 16 bytes, got %d", len(b))
	}
	return uuid.UUID(b), nil
}

// ApplyPagination validates and normalises list options.
// It returns the effective limit, an optional afterID decoded from the cursor,
// or an error if the cursor is malformed.
func ApplyPagination(opts port.ListOptions) (limit int, afterID *uuid.UUID, err error) {
	limit = opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	if opts.Cursor != "" {
		id, err := DecodeCursor(opts.Cursor)
		if err != nil {
			return 0, nil, err
		}
		afterID = &id
	}

	return limit, afterID, nil
}
