package domain

import (
	"time"

	"github.com/google/uuid"
)

type AdapterType string

const (
	AdapterTypeSES  AdapterType = "ses"
	AdapterTypeSMTP AdapterType = "smtp"
)

type Adapter struct {
	ID              uuid.UUID
	WorkspaceID     *uuid.UUID // nil = global
	Name            string
	AdapterType     AdapterType
	ConfigEncrypted []byte // AES-256-GCM encrypted JSON
	IsDefault       bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}
