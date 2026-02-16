package dkim

import "fmt"

// DNSRecord returns the DNS TXT record hostname for DKIM.
// Format: selector._domainkey.domain
func DNSRecord(selector, domain string) string {
	return fmt.Sprintf("%s._domainkey.%s", selector, domain)
}

// DNSTXTValue returns the DKIM DNS TXT record value.
// Format: v=DKIM1; k=rsa; p=<publicKeyBase64>
func DNSTXTValue(publicKeyBase64 string) string {
	return fmt.Sprintf("v=DKIM1; k=rsa; p=%s", publicKeyBase64)
}
