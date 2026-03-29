package postgres

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/port"
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

// EncodeTimeCursor encodes a (time, UUID) pair as a base64 cursor for
// partitioned tables ordered by (created_at DESC, id DESC).
func EncodeTimeCursor(t time.Time, id uuid.UUID) string {
	// 8 bytes for unix nanos + 16 bytes for UUID = 24 bytes
	buf := make([]byte, 24)
	binary.BigEndian.PutUint64(buf[:8], uint64(t.UnixNano()))
	copy(buf[8:], id[:])
	return base64.RawURLEncoding.EncodeToString(buf)
}

// DecodeTimeCursor decodes a composite cursor back to (time, UUID).
func DecodeTimeCursor(cursor string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid time cursor: %w", err)
	}
	if len(b) != 24 {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid time cursor: expected 24 bytes, got %d", len(b))
	}
	nanos := int64(binary.BigEndian.Uint64(b[:8]))
	t := time.Unix(0, nanos).UTC()
	id := uuid.UUID(b[8:])
	return t, id, nil
}

// NormalizeLimit clamps a requested limit to [1, maxLimit], defaulting to defaultLimit.
func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
