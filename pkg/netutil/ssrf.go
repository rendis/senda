// Package netutil provides network security utilities for SSRF prevention.
package netutil

import "net"

// IsPrivateOrReservedHost resolves the hostname and checks whether any of its
// IPs are loopback, private, link-local, unspecified, or point to well-known
// cloud metadata endpoints. DNS resolution failures are treated as blocked to be safe.
//
// This function should be called both at webhook URL validation time (creation/update)
// and at delivery time (in webhook_worker.go) to guard against DNS rebinding attacks.
func IsPrivateOrReservedHost(hostname string) bool {
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return true // block if DNS fails
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}

		// Block unspecified addresses (0.0.0.0, ::).
		if ip.IsUnspecified() {
			return true
		}

		// Block AWS metadata endpoint (169.254.169.254).
		if ip.Equal(net.ParseIP("169.254.169.254")) {
			return true
		}

		// Block AWS IMDSv2 IPv6 endpoint.
		if ip.Equal(net.ParseIP("fd00:ec2::254")) {
			return true
		}

		// Block CGNAT / Shared Address Space (RFC 6598: 100.64.0.0/10).
		cgnat := net.IPNet{IP: net.ParseIP("100.64.0.0"), Mask: net.CIDRMask(10, 32)}
		if cgnat.Contains(ip) {
			return true
		}
	}
	return false
}
