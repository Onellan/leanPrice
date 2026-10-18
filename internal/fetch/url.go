package fetch

import (
	"errors"
	"net"
	"net/netip"
	"strings"
)

var deniedPrefixes = parsePrefixes([]string{
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
	"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
	"192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24",
	"203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "::ffff:0:0/96", "64:ff9b:1::/48", "100::/64",
	"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20", "fc00::/7",
	"fe80::/10", "ff00::/8",
})

func parsePrefixes(values []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

func validateResolvedIPs(ips []net.IPAddr) error {
	if len(ips) == 0 {
		return errors.New("retailer host has no IP addresses")
	}
	for _, result := range ips {
		if result.Zone != "" {
			return ErrUnsafeAddress
		}
		addr, ok := netip.AddrFromSlice(result.IP)
		if !ok {
			return ErrUnsafeAddress
		}
		addr = addr.Unmap()
		if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
			return ErrUnsafeAddress
		}
		for _, prefix := range deniedPrefixes {
			if prefix.Contains(addr) {
				return ErrUnsafeAddress
			}
		}
	}
	return nil
}

// ValidatePublicIPs rejects private, local, special-purpose and mixed DNS
// answers before a caller can use them for outbound requests.
func ValidatePublicIPs(ips []net.IPAddr) error { return validateResolvedIPs(ips) }

func validDNSName(host string) bool {
	if len(host) > 253 || strings.ContainsAny(host, "\x00/%\\") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
