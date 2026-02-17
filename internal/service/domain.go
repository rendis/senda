package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/adapter/dkim"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// DomainService handles domain registration and verification orchestration.
type DomainService struct {
	store    port.DomainStore
	crypto   port.Crypto
	jobQueue port.JobQueue
}

// NewDomainService creates a new DomainService.
func NewDomainService(store port.DomainStore, crypto port.Crypto, jq port.JobQueue) *DomainService {
	return &DomainService{store: store, crypto: crypto, jobQueue: jq}
}

// Register generates DKIM keys, encrypts the private key, and creates a new domain record.
func (s *DomainService) Register(ctx context.Context, domainName string, workspaceID *uuid.UUID) (*domain.Domain, error) {
	privateKeyPEM, publicKeyBase64, err := dkim.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	encryptedPrivKey, err := s.crypto.Encrypt(privateKeyPEM)
	if err != nil {
		return nil, err
	}

	selector := "senda"
	dnsRecords := []map[string]any{
		{
			"type":  "TXT",
			"host":  dkim.DNSRecord(selector, domainName),
			"value": dkim.DNSTXTValue(publicKeyBase64),
		},
	}

	now := time.Now().UTC()
	d := &domain.Domain{
		ID:                      uuid.Must(uuid.NewV7()),
		WorkspaceID:             workspaceID,
		DomainName:              domainName,
		DKIMSelector:            selector,
		DKIMPublicKey:           publicKeyBase64,
		DKIMPrivateKeyEncrypted: encryptedPrivKey,
		DNSRecords:              dnsRecords,
		Status:                  domain.DomainStatusPending,
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	if err := s.store.Create(ctx, d); err != nil {
		return nil, err
	}

	return d, nil
}

// RequestVerification enqueues a domain DNS verification check job.
func (s *DomainService) RequestVerification(ctx context.Context, domainID uuid.UUID) error {
	return s.jobQueue.EnqueueDomainCheck(ctx, domainID)
}
