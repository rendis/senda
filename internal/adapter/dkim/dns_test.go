package dkim_test

import (
	"testing"

	"github.com/senda-app/senda/internal/adapter/dkim"
)

func TestDNSRecord(t *testing.T) {
	got := dkim.DNSRecord("selector1", "example.com")
	want := "selector1._domainkey.example.com"

	if got != want {
		t.Errorf("DNSRecord() = %q, want %q", got, want)
	}
}

func TestDNSTXTValue(t *testing.T) {
	got := dkim.DNSTXTValue("MIGfMA0GCSqGSIb3DQEBAQUAA4G")
	want := "v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4G"

	if got != want {
		t.Errorf("DNSTXTValue() = %q, want %q", got, want)
	}
}
