// Package notify sends one bounded webhook protocol through an exact,
// administrator-configured notification target policy.
package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Onellan/leanPrice/internal/extract"
	"github.com/Onellan/leanPrice/internal/fetch"
	"github.com/Onellan/leanPrice/internal/pricing"
)

const MaxMessageBytes = 8 << 10
const MaxResponseBytes = 8 << 10

type Response struct {
	StatusCode int
	RetryAfter time.Duration
}

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type Config struct {
	Endpoint            string
	BearerToken         string
	AllowedPrivateCIDRs []netip.Prefix
	Resolver            Resolver
	Timeout             time.Duration
	TLSRootCAs          *x509.CertPool
}

type Message struct {
	IdempotencyKey string `json:"idempotency_key"`
	Product        string `json:"product"`
	Retailer       string `json:"retailer"`
	AmountMinor    int64  `json:"amount_minor"`
	Currency       string `json:"currency"`
	Reason         string `json:"reason"`
	ObservedAt     int64  `json:"observed_at"`
	Stock          string `json:"stock"`
	URL            string `json:"url,omitempty"`
}

// TestMessage is a deliberately non-alerting payload used only by an explicit
// administrator action to verify the configured notification destination.
type TestMessage struct {
	Type           string `json:"type"`
	IdempotencyKey string `json:"idempotency_key"`
	Message        string `json:"message"`
}

type Client struct {
	endpoint *url.URL
	token    string
	private  []netip.Prefix
	resolver Resolver
	timeout  time.Duration
	rootCAs  *x509.CertPool
}

func NewClient(config Config) (*Client, error) {
	if len(config.Endpoint) == 0 || len(config.Endpoint) > 2048 {
		return nil, errors.New("notify: exact webhook endpoint is required")
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint == nil || endpoint.User != nil || endpoint.Fragment != "" || endpoint.Hostname() == "" {
		return nil, errors.New("notify: webhook endpoint is malformed")
	}
	if endpoint.Scheme != "https" && endpoint.Scheme != "http" {
		return nil, errors.New("notify: webhook endpoint must use HTTPS")
	}
	if endpoint.Port() != "" {
		port, parseErr := strconv.Atoi(endpoint.Port())
		if parseErr != nil || port < 1 || port > 65535 {
			return nil, errors.New("notify: webhook port is invalid")
		}
	}
	if endpoint.Scheme == "https" && endpoint.Port() != "" && endpoint.Port() != "443" && len(config.AllowedPrivateCIDRs) == 0 {
		return nil, errors.New("notify: public HTTPS webhooks use port 443")
	}
	if endpoint.Scheme == "http" && len(config.AllowedPrivateCIDRs) == 0 {
		return nil, errors.New("notify: HTTP is allowed only for an explicitly allowlisted private target")
	}
	for _, prefix := range config.AllowedPrivateCIDRs {
		if !prefix.IsValid() || !prefix.Masked().Addr().IsPrivate() && !prefix.Masked().Addr().IsLoopback() {
			return nil, errors.New("notify: private webhook CIDRs must be private or loopback networks")
		}
	}
	if config.Resolver == nil {
		config.Resolver = net.DefaultResolver
	}
	if config.Timeout <= 0 || config.Timeout > 30*time.Second {
		config.Timeout = 5 * time.Second
	}
	return &Client{endpoint: endpoint, token: config.BearerToken, private: append([]netip.Prefix(nil), config.AllowedPrivateCIDRs...), resolver: config.Resolver, timeout: config.Timeout, rootCAs: config.TLSRootCAs}, nil
}

func (c *Client) Send(ctx context.Context, idempotencyKey string, message Message) (Response, error) {
	if c == nil || c.endpoint == nil || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 200 {
		return Response{}, errors.New("notify: client and bounded idempotency key are required")
	}
	message.IdempotencyKey = idempotencyKey
	if message.URL != "" {
		message.URL = extract.SanitizeURL(message.URL)
	}
	if err := validateMessage(message); err != nil {
		return Response{}, err
	}
	body, err := json.Marshal(message)
	if err != nil {
		return Response{}, errors.New("notify: webhook message exceeds its size limit")
	}
	return c.sendJSON(ctx, idempotencyKey, body)
}

// SendTest verifies the configured destination with a fixed, non-price payload.
// It follows the same destination policy, timeout, redirect and response caps
// as alert delivery.
func (c *Client) SendTest(ctx context.Context, idempotencyKey string) (Response, error) {
	if c == nil || c.endpoint == nil || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 200 {
		return Response{}, errors.New("notify: client and bounded idempotency key are required")
	}
	body, err := json.Marshal(TestMessage{Type: "test", IdempotencyKey: idempotencyKey, Message: "leanPrice test notification"})
	if err != nil {
		return Response{}, errors.New("notify: webhook test message could not be encoded")
	}
	return c.sendJSON(ctx, idempotencyKey, body)
}

func (c *Client) sendJSON(ctx context.Context, idempotencyKey string, body []byte) (Response, error) {
	if len(body) > MaxMessageBytes {
		return Response{}, errors.New("notify: webhook message exceeds its size limit")
	}
	operation, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(operation, http.MethodPost, c.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return Response{}, errors.New("notify: create webhook request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain;q=0.5")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	ips, err := c.resolveAllowed(operation)
	if err != nil {
		return Response{}, fmt.Errorf("notify: target policy rejected destination: %w", err)
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, splitErr := net.SplitHostPort(address)
			if splitErr != nil || !strings.EqualFold(strings.TrimSuffix(host, "."), c.endpoint.Hostname()) || port != c.port() {
				return nil, errors.New("notify: destination changed during connection")
			}
			dialer := net.Dialer{Timeout: c.timeout, KeepAlive: -1}
			var lastErr error
			for _, ip := range ips {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), c.port()))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			if lastErr == nil {
				lastErr = errors.New("notify: target resolved to no usable addresses")
			}
			return nil, lastErr
		},
		TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: c.rootCAs},
		ForceAttemptHTTP2: true, DisableKeepAlives: true,
		TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second,
		MaxResponseHeaderBytes: 16 << 10,
	}
	client := &http.Client{Transport: transport, Timeout: c.timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	transport.CloseIdleConnections()
	if err != nil {
		return Response{}, errors.New("notify: webhook request failed")
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if readErr != nil {
		return Response{}, errors.New("notify: webhook response could not be read")
	}
	if len(responseBody) > MaxResponseBytes {
		return Response{}, errors.New("notify: webhook response exceeds its size limit")
	}
	return Response{StatusCode: response.StatusCode, RetryAfter: parseRetryAfter(response.Header.Get("Retry-After"), time.Now())}, nil
}

func (c *Client) resolveAllowed(ctx context.Context) ([]netip.Addr, error) {
	host := strings.TrimSuffix(strings.ToLower(c.endpoint.Hostname()), ".")
	if host == "" {
		return nil, errors.New("empty target host")
	}
	resolved, err := c.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(resolved) == 0 {
		return nil, errors.New("target resolution failed")
	}
	ips := make([]netip.Addr, 0, len(resolved))
	for _, result := range resolved {
		if result.Zone != "" {
			return nil, errors.New("zoned address is not allowed")
		}
		ip, ok := netip.AddrFromSlice(result.IP)
		if !ok {
			return nil, errors.New("invalid target address")
		}
		ip = ip.Unmap()
		if c.endpoint.Scheme == "http" && !ip.IsPrivate() && !ip.IsLoopback() {
			return nil, errors.New("unencrypted webhooks require an explicitly allowlisted private address")
		}
		if c.endpoint.Port() != "" && c.endpoint.Port() != "443" && !ip.IsPrivate() && !ip.IsLoopback() {
			return nil, errors.New("non-standard public webhook ports are not allowed")
		}
		if ip.IsPrivate() || ip.IsLoopback() {
			allowed := false
			for _, prefix := range c.private {
				if prefix.IsValid() && prefix.Contains(ip) {
					allowed = true
					break
				}
			}
			if !allowed || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
				return nil, errors.New("private address is not explicitly allowlisted")
			}
		} else if err := fetch.ValidatePublicIPs([]net.IPAddr{{IP: net.IP(ip.AsSlice())}}); err != nil {
			return nil, errors.New("non-public address is not allowed")
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

func (c *Client) port() string {
	if port := c.endpoint.Port(); port != "" {
		return port
	}
	if c.endpoint.Scheme == "https" {
		return "443"
	}
	return "80"
}

func validateMessage(message Message) error {
	if len(message.IdempotencyKey) > 200 || len(message.Product) == 0 || len(message.Product) > 240 || len(message.Retailer) == 0 || len(message.Retailer) > 120 ||
		message.AmountMinor <= 0 || len(message.Currency) != 3 || message.Currency != strings.ToUpper(message.Currency) || len(message.Reason) == 0 || len(message.Reason) > 80 ||
		message.ObservedAt <= 0 || message.Stock != "in_stock" && message.Stock != "unknown" {
		return errors.New("notify: webhook message is invalid")
	}
	if _, ok := pricing.MinorUnitScale(message.Currency); !ok {
		return errors.New("notify: webhook message currency is unsupported")
	}
	if message.URL != "" {
		parsed, err := url.Parse(message.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("notify: webhook product URL is invalid")
		}
	}
	return nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		if seconds > int64((24*time.Hour)/time.Second) {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		if delay := at.Sub(now); delay > 24*time.Hour {
			return 24 * time.Hour
		} else {
			return delay
		}
	}
	return 0
}
