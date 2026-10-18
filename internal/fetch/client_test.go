package fetch

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type testPolicy map[string]bool

func (p testPolicy) HostApproved(_ context.Context, host string) (bool, error) {
	return p[strings.ToLower(host)], nil
}

type testResolver struct {
	ips map[string][]net.IPAddr
	err error
}

func (r testResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.ips[strings.ToLower(host)], nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func testClient(t *testing.T, resolver Resolver, rt http.RoundTripper) *Client {
	t.Helper()
	client, err := NewClient(Config{
		Policy:       testPolicy{"retailer.test": true},
		Resolver:     resolver,
		MaxBodyBytes: 32,
	})
	if err != nil {
		t.Fatalf("NewClient(): %v", err)
	}
	client.config.HostInterval = 0
	if rt != nil {
		client.http.Transport = rt
	}
	return client
}

func publicResolver() Resolver {
	return testResolver{ips: map[string][]net.IPAddr{
		"retailer.test": {{IP: net.ParseIP("8.8.8.8")}},
	}}
}

func TestFetchRequiresPolicyAndRejectsUnsafeTargets(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("NewClient without host policy succeeded")
	}
	c := testClient(t, publicResolver(), nil)
	cases := []string{
		"http://retailer.test/product",
		"https://retailer.test:8443/product",
		"https://user:password@retailer.test/product",
		"file:///etc/passwd",
		"https://8.8.8.8/product",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			_, err := c.Fetch(context.Background(), raw)
			var fetchErr *FetchError
			if !errors.As(err, &fetchErr) || fetchErr.Kind != KindPolicy {
				t.Fatalf("Fetch() error = %v, want policy error", err)
			}
		})
	}
}

func TestValidateTargetRejectsPrivateAndMixedDNSAnswers(t *testing.T) {
	cases := map[string][]net.IPAddr{
		"loopback":        {{IP: net.ParseIP("127.0.0.1")}},
		"rfc1918":         {{IP: net.ParseIP("10.1.2.3")}},
		"link_local":      {{IP: net.ParseIP("169.254.169.254")}},
		"carrier_nat":     {{IP: net.ParseIP("100.64.0.1")}},
		"documentation":   {{IP: net.ParseIP("203.0.113.4")}},
		"ipv6_ula":        {{IP: net.ParseIP("fd00::1")}},
		"ipv6_link_local": {{IP: net.ParseIP("fe80::1")}},
		"mixed_rebinding": {{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("192.168.1.2")}},
	}
	for name, ips := range cases {
		t.Run(name, func(t *testing.T) {
			c := testClient(t, testResolver{ips: map[string][]net.IPAddr{"retailer.test": ips}}, nil)
			u, _ := url.Parse("https://retailer.test/product")
			if err := c.validateTarget(context.Background(), u); !errors.Is(err, ErrUnsafeAddress) {
				t.Fatalf("validateTarget() = %v, want ErrUnsafeAddress", err)
			}
		})
	}
}

func TestSafeDialerRejectsPrivateResolutionBeforeDial(t *testing.T) {
	dialer := safeDialer{resolver: testResolver{ips: map[string][]net.IPAddr{
		"retailer.test": {{IP: net.ParseIP("127.0.0.1")}},
	}}}
	if _, err := dialer.DialContext(context.Background(), "tcp", "retailer.test:443"); !errors.Is(err, ErrUnsafeAddress) {
		t.Fatalf("DialContext() = %v, want ErrUnsafeAddress", err)
	}
}

func TestFetchCapsResponseAndDoesNotReturnQueryString(t *testing.T) {
	c := testClient(t, publicResolver(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("abc")),
			Request:    req,
		}, nil
	}))
	result, err := c.Fetch(context.Background(), "https://retailer.test/path?token=secret")
	if err != nil {
		t.Fatalf("Fetch(): %v", err)
	}
	if result.FinalURL != "https://retailer.test/" {
		t.Fatalf("FinalURL = %q, query/path was not sanitized", result.FinalURL)
	}
	if result.Digest == "" || string(result.Body) != "abc" {
		t.Fatalf("incomplete fetch result: %#v", result)
	}

	c = testClient(t, publicResolver(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 33))), Request: req}, nil
	}))
	if _, err := c.Fetch(context.Background(), "https://retailer.test/product"); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("oversized Fetch() = %v, want ErrBodyTooLarge", err)
	}
}

func TestRedirectToUnapprovedHostIsRejected(t *testing.T) {
	called := 0
	var hosts []string
	c := testClient(t, publicResolver(), roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called++
		hosts = append(hosts, req.URL.Host)
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://internal.test/private"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	}))
	internalURL, _ := url.Parse("https://internal.test/private")
	if err := c.validateTarget(context.Background(), internalURL); !errors.Is(err, ErrUnapprovedHost) {
		t.Fatalf("direct validation for redirect target = %v", err)
	}
	redirectReq, _ := http.NewRequest(http.MethodGet, "https://internal.test/private", nil)
	if err := c.http.CheckRedirect(redirectReq, []*http.Request{{URL: &url.URL{Host: "retailer.test"}}}); !errors.Is(err, ErrUnapprovedHost) {
		t.Fatalf("direct redirect callback = %v", err)
	}
	_, err := c.Fetch(context.Background(), "https://retailer.test/product")
	var fetchErr *FetchError
	if !errors.As(err, &fetchErr) || fetchErr.Kind != KindPolicy {
		t.Fatalf("Fetch() = %#v, unwrap=%v, hosts=%v, want policy error", fetchErr, fetchErr.Err, hosts)
	}
	if called != 1 {
		t.Fatalf("round trip count = %d, want redirect blocked before second request", called)
	}
}

func TestRetryAfterAndErrorMessagesAreBounded(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	if got := retryAfter("45", now); got != 45*time.Second {
		t.Fatalf("retryAfter seconds = %v", got)
	}
	if got := retryAfter(now.Add(time.Minute).Format(http.TimeFormat), now); got != time.Minute {
		t.Fatalf("retryAfter date = %v", got)
	}
	if got := (&FetchError{Kind: KindUnavailable, Err: errors.New("https://retailer.test/?api_key=secret")}).Error(); got != string(KindUnavailable) {
		t.Fatalf("FetchError leaked details: %q", got)
	}
}
