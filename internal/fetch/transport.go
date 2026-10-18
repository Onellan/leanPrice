package fetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

type safeRoundTripper struct {
	base     http.RoundTripper
	policy   HostPolicy
	resolver Resolver
}

func (t *safeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil || req.URL.Scheme != "https" || req.URL.Port() != "" && req.URL.Port() != "443" {
		return nil, ErrUnsafeURL
	}
	host := strings.TrimSuffix(strings.ToLower(req.URL.Hostname()), ".")
	approved, err := t.policy.HostApproved(req.Context(), host)
	if err != nil {
		return nil, fmt.Errorf("check retailer source policy: %w", err)
	}
	if !approved {
		return nil, fmt.Errorf("%w: %s", ErrUnapprovedHost, host)
	}
	ips, err := t.resolver.LookupIPAddr(req.Context(), host)
	if err != nil {
		return nil, fmt.Errorf("resolve retailer host: %w", err)
	}
	if err := validateResolvedIPs(ips); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

type safeDialer struct {
	resolver Resolver
	dialer   net.Dialer
}

func (d *safeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, ErrUnsafeURL
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if !validDNSName(host) {
		return nil, ErrUnsafeURL
	}
	ips, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve validated retailer host: %w", err)
	}
	if err := validateResolvedIPs(ips); err != nil {
		return nil, err
	}
	var lastErr error
	for _, result := range ips {
		ip, ok := netip.AddrFromSlice(result.IP)
		if !ok {
			continue
		}
		conn, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(ip.Unmap().String(), strconv.Itoa(443)))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = ErrUnsafeAddress
	}
	return nil, lastErr
}
