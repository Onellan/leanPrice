// Package fetch is the only authority for outbound retailer HTTP requests.
// Callers provide a source-policy implementation; unapproved hosts fail closed.
package fetch

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultTimeout      = 15 * time.Second
	DefaultBodyLimit    = 2 << 20
	DefaultRedirects    = 3
	DefaultHostInterval = 10 * time.Second
	maximumOperation    = 30 * time.Second
	maximumCooldown     = 24 * time.Hour
)

var (
	ErrUnapprovedHost = errors.New("retailer host is not approved")
	ErrUnsafeURL      = errors.New("retailer URL is unsafe")
	ErrUnsafeAddress  = errors.New("retailer host resolved to a non-public address")
	ErrBodyTooLarge   = errors.New("retailer response exceeds the decompressed body limit")
)

type HostPolicy interface {
	HostApproved(context.Context, string) (bool, error)
}

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type RateGate interface {
	ReserveHost(context.Context, string, time.Time, time.Duration) (time.Duration, error)
	DeferHost(context.Context, string, time.Time) error
}

type Config struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	MaxRedirects int
	HostInterval time.Duration
	Policy       HostPolicy
	RateGate     RateGate
	Resolver     Resolver
	Now          func() time.Time
}

type Result struct {
	StatusCode  int
	ContentType string
	FinalURL    string
	RetrievedAt time.Time
	Digest      string
	Body        []byte
}

type ErrorKind string

const (
	KindPolicy      ErrorKind = "policy_disallowed"
	KindBlocked     ErrorKind = "blocked"
	KindRateLimited ErrorKind = "rate_limited"
	KindUnavailable ErrorKind = "offline"
	KindTimeout     ErrorKind = "timeout"
	KindOversized   ErrorKind = "oversized"
	KindInvalid     ErrorKind = "invalid_response"
)

type FetchError struct {
	Kind       ErrorKind
	StatusCode int
	RetryAfter time.Duration
	Err        error
}

func (e *FetchError) Error() string {
	return string(e.Kind)
}

func (e *FetchError) Unwrap() error { return e.Err }

type hostState struct {
	nextStart  time.Time
	blockedTil time.Time
}

type Client struct {
	config   Config
	resolver Resolver
	hosts    HostPolicy
	http     *http.Client
	global   chan struct{}
	mu       sync.Mutex
	state    map[string]hostState
}

func NewClient(config Config) (*Client, error) {
	if config.Policy == nil {
		return nil, errors.New("retailer host policy is required")
	}
	if config.Timeout <= 0 || config.Timeout > maximumOperation {
		config.Timeout = DefaultTimeout
	}
	if config.MaxBodyBytes <= 0 || config.MaxBodyBytes > DefaultBodyLimit {
		config.MaxBodyBytes = DefaultBodyLimit
	}
	if config.MaxRedirects <= 0 || config.MaxRedirects > DefaultRedirects {
		config.MaxRedirects = DefaultRedirects
	}
	if config.HostInterval < DefaultHostInterval {
		config.HostInterval = DefaultHostInterval
	}
	if config.Resolver == nil {
		config.Resolver = net.DefaultResolver
	}
	if config.Now == nil {
		config.Now = time.Now
	}

	c := &Client{
		config:   config,
		resolver: config.Resolver,
		hosts:    config.Policy,
		global:   make(chan struct{}, 1),
		state:    make(map[string]hostState),
	}
	dialer := &safeDialer{resolver: c.resolver}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           2,
		MaxIdleConnsPerHost:    1,
		MaxConnsPerHost:        1,
		IdleConnTimeout:        30 * time.Second,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  5 * time.Second,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
	c.http = &http.Client{
		Transport: &safeRoundTripper{base: transport, policy: c.hosts, resolver: c.resolver},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > config.MaxRedirects {
				return errors.New("too many retailer redirects")
			}
			if err := c.validateTarget(req.Context(), req.URL); err != nil {
				return err
			}
			host := strings.TrimSuffix(strings.ToLower(req.URL.Hostname()), ".")
			if err := c.waitForHost(req.Context(), host); err != nil {
				return err
			}
			return nil
		},
	}
	return c, nil
}

func (c *Client) Fetch(ctx context.Context, rawURL string) (Result, error) {
	operationCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	target, err := url.Parse(rawURL)
	if err != nil {
		return Result{}, &FetchError{Kind: KindPolicy, Err: fmt.Errorf("parse URL: %w", ErrUnsafeURL)}
	}
	if err := c.validateTarget(operationCtx, target); err != nil {
		return Result{}, classifyPolicyError(err)
	}
	host := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	target.Host = host
	if port := target.Port(); port != "" {
		target.Host = net.JoinHostPort(host, port)
	}
	select {
	case c.global <- struct{}{}:
		defer func() { <-c.global }()
	case <-operationCtx.Done():
		return Result{}, &FetchError{Kind: KindTimeout, Err: operationCtx.Err()}
	}
	if err := c.waitForHost(operationCtx, host); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(operationCtx.Err(), context.DeadlineExceeded) {
			return Result{}, &FetchError{Kind: KindTimeout, Err: err}
		}
		return Result{}, &FetchError{Kind: KindUnavailable, Err: err}
	}

	req, err := http.NewRequestWithContext(operationCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Result{}, &FetchError{Kind: KindPolicy, Err: fmt.Errorf("create request: %w", ErrUnsafeURL)}
	}
	req.Header.Set("Accept", "text/html,application/ld+json,application/json;q=0.9,*/*;q=0.1")
	req.Header.Set("User-Agent", "leanPrice/1.0 (+local household price monitoring)")

	response, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrUnapprovedHost) || errors.Is(err, ErrUnsafeURL) || errors.Is(err, ErrUnsafeAddress) {
			return Result{}, classifyPolicyError(err)
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(operationCtx.Err(), context.DeadlineExceeded) {
			return Result{}, &FetchError{Kind: KindTimeout, Err: err}
		}
		return Result{}, &FetchError{Kind: KindUnavailable, Err: err}
	}
	defer response.Body.Close()

	result := Result{
		StatusCode:  response.StatusCode,
		ContentType: response.Header.Get("Content-Type"),
		FinalURL:    response.Request.URL.Scheme + "://" + response.Request.URL.Host + "/",
		RetrievedAt: c.config.Now().UTC(),
	}
	if response.StatusCode == http.StatusForbidden {
		return result, &FetchError{Kind: KindBlocked, StatusCode: response.StatusCode}
	}
	if response.StatusCode == http.StatusTooManyRequests {
		cooldown := retryAfter(response.Header.Get("Retry-After"), c.config.Now())
		if cooldown == 0 {
			cooldown = 30 * time.Second
		}
		c.blockHost(host, cooldown)
		if c.config.RateGate != nil {
			if err := c.config.RateGate.DeferHost(operationCtx, host, c.config.Now().Add(cooldown)); err != nil {
				return result, &FetchError{Kind: KindUnavailable, StatusCode: response.StatusCode, Err: err}
			}
		}
		return result, &FetchError{Kind: KindRateLimited, StatusCode: response.StatusCode, RetryAfter: cooldown}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, &FetchError{Kind: KindUnavailable, StatusCode: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, c.config.MaxBodyBytes+1))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(operationCtx.Err(), context.DeadlineExceeded) {
			return result, &FetchError{Kind: KindTimeout, Err: err}
		}
		return result, &FetchError{Kind: KindInvalid, Err: err}
	}
	if int64(len(body)) > c.config.MaxBodyBytes {
		return result, &FetchError{Kind: KindOversized, Err: ErrBodyTooLarge}
	}
	digest := sha256.Sum256(body)
	result.Digest = hex.EncodeToString(digest[:])
	result.Body = body
	return result, nil
}

func (c *Client) validateTarget(ctx context.Context, target *url.URL) error {
	if target == nil || target.Scheme != "https" || target.Opaque != "" || target.User != nil || target.Host == "" {
		return ErrUnsafeURL
	}
	host := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	if host == "" || !validDNSName(host) || net.ParseIP(host) != nil {
		return ErrUnsafeURL
	}
	if target.Port() != "" && target.Port() != "443" {
		return ErrUnsafeURL
	}
	approved, err := c.hosts.HostApproved(ctx, host)
	if err != nil {
		return fmt.Errorf("check retailer source policy: %w", err)
	}
	if !approved {
		return fmt.Errorf("%w: %s", ErrUnapprovedHost, host)
	}
	ips, err := c.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve retailer host: %w", err)
	}
	return validateResolvedIPs(ips)
}

func (c *Client) waitForHost(ctx context.Context, host string) error {
	if c.config.RateGate != nil {
		delay, err := c.config.RateGate.ReserveHost(ctx, host, c.config.Now(), c.config.HostInterval)
		if err != nil {
			return err
		}
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for {
		c.mu.Lock()
		now := c.config.Now()
		state := c.state[host]
		waitUntil := state.nextStart
		if state.blockedTil.After(waitUntil) {
			waitUntil = state.blockedTil
		}
		if !waitUntil.After(now) {
			state.nextStart = now.Add(c.config.HostInterval)
			c.state[host] = state
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		delay := waitUntil.Sub(now)
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

func (c *Client) blockHost(host string, delay time.Duration) {
	if delay <= 0 {
		return
	}
	if delay > maximumCooldown {
		delay = maximumCooldown
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.state[host]
	blockedTil := c.config.Now().Add(delay)
	if blockedTil.After(state.blockedTil) {
		state.blockedTil = blockedTil
	}
	c.state[host] = state
}

func classifyPolicyError(err error) error {
	kind := KindPolicy
	if errors.Is(err, ErrUnsafeURL) || errors.Is(err, ErrUnsafeAddress) {
		kind = KindPolicy
	}
	if errors.Is(err, ErrUnapprovedHost) {
		kind = KindPolicy
	}
	return &FetchError{Kind: kind, Err: err}
}

func retryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		if seconds > int64(maximumCooldown/time.Second) {
			return maximumCooldown
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}
