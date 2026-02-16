package domain

import (
	"time"

	"github.com/google/uuid"
)

type DomainStatus string

const (
	DomainStatusPending  DomainStatus = "pending"
	DomainStatusVerified DomainStatus = "verified"
	DomainStatusError    DomainStatus = "error"
)

type Domain struct {
	ID                      uuid.UUID
	WorkspaceID             *uuid.UUID // nil = global
	DomainName              string     // e.g., "example.com"
	DKIMSelector            string     // e.g., "senda"
	DKIMPublicKey           string
	DKIMPrivateKeyEncrypted []byte // AES-256-GCM
	DNSRecords              []map[string]any // JSON: DNS records for domain verification
	Status                  DomainStatus
	VerifiedAt              *time.Time
	LastCheckAt             *time.Time
	NextCheckAt             *time.Time
	LastError               *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	DeletedAt               *time.Time
}
