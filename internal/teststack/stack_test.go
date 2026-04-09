package teststack

import "testing"

func TestOIDCDiscoveryURL_UsesInternalKeycloakAlias(t *testing.T) {
	got := oidcDiscoveryURL()
	want := "http://keycloak:8080/realms/senda/.well-known/openid-configuration"

	if got != want {
		t.Fatalf("oidcDiscoveryURL() = %q, want %q", got, want)
	}
}
