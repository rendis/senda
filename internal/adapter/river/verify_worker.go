package river

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	goriver "github.com/riverqueue/river"

	"github.com/senda-app/senda/internal/adapter/dkim"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/port"
)

// DNSResolver abstracts DNS lookups for testability.
type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// NetDNSResolver uses net.Resolver for real DNS lookups.
type NetDNSResolver struct{}

func (NetDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

// VerifyWorker processes domain DNS verification jobs.
type VerifyWorker struct {
	goriver.WorkerDefaults[VerifyJobArgs]

	domainStore port.DomainStore
	dns         DNSResolver
}

// NewVerifyWorker creates a new domain verification worker.
func NewVerifyWorker(domainStore port.DomainStore, dns DNSResolver) *VerifyWorker {
	if dns == nil {
		dns = NetDNSResolver{}
	}
	return &VerifyWorker{
		domainStore: domainStore,
		dns:         dns,
	}
}

// Work processes a single domain verification job.
func (w *VerifyWorker) Work(ctx context.Context, job *goriver.Job[VerifyJobArgs]) error {
	args := job.Args

	// 1. Get domain by ID.
	d, err := w.domainStore.GetByID(ctx, args.DomainID)
	if err != nil {
		return goriver.JobCancel(fmt.Errorf("verify: domain not found id=%s: %w", args.DomainID, err))
	}

	now := time.Now().UTC()
	nextCheck := now.Add(24 * time.Hour)

	// 2. Check DKIM DNS records (TXT lookup).
	dkimHost := dkim.DNSRecord(d.DKIMSelector, d.DomainName)
	expectedValue := dkim.DNSTXTValue(d.DKIMPublicKey)

	records, err := w.dns.LookupTXT(ctx, dkimHost)
	if err != nil {
		// DNS lookup failed — mark as error, schedule next check.
		errMsg := fmt.Sprintf("DNS lookup failed for %s: %v", dkimHost, err)
		d.Status = domain.DomainStatusError
		d.LastError = &errMsg
		d.LastCheckAt = &now
		d.NextCheckAt = &nextCheck
		if updateErr := w.domainStore.Update(ctx, d); updateErr != nil {
			return fmt.Errorf("verify: update domain after dns error: %w", updateErr)
		}
		// Return nil — the DNS failure is recorded, no need to retry the job.
		return nil
	}

	// 3. Check if expected DKIM value is present in TXT records.
	verified := false
	for _, record := range records {
		if strings.TrimSpace(record) == strings.TrimSpace(expectedValue) {
			verified = true
			break
		}
	}

	// 4. Update domain status.
	if verified {
		d.Status = domain.DomainStatusVerified
		d.VerifiedAt = &now
		d.LastError = nil
	} else {
		errMsg := fmt.Sprintf("DKIM record not found at %s; got %v", dkimHost, records)
		d.Status = domain.DomainStatusError
		d.LastError = &errMsg
	}

	d.LastCheckAt = &now
	d.NextCheckAt = &nextCheck

	if err := w.domainStore.Update(ctx, d); err != nil {
		return fmt.Errorf("verify: update domain status: %w", err)
	}

	return nil
}
