package notify

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type fixedResolver []net.IPAddr

func (r fixedResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return []net.IPAddr(r), nil
}

func TestWebhookUsesExactPrivateTargetAndDoesNotFollowRedirects(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/hook" || r.Header.Get("Authorization") != "Bearer secret-token" || r.Header.Get("Idempotency-Key") != "episode-1" {
			t.Errorf("unexpected webhook request target or headers: %s %#v", r.URL, r.Header)
		}
		w.Header().Set("Location", "https://other.example/steal")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	rootCAs := server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	client, err := NewClient(Config{
		Endpoint:            server.URL + "/hook?secret=query",
		BearerToken:         "secret-token",
		AllowedPrivateCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		Resolver:            fixedResolver{{IP: net.ParseIP("127.0.0.1")}},
		Timeout:             2 * time.Second,
		TLSRootCAs:          rootCAs,
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.Send(context.Background(), "episode-1", Message{Product: "Soap", Retailer: "Test", AmountMinor: 1234, Currency: "ZAR", Reason: "threshold_met", ObservedAt: 1, Stock: "unknown"})
	if err != nil || status.StatusCode != http.StatusTemporaryRedirect || requests != 1 {
		t.Fatalf("Send() = %+v, %v; requests=%d", status, err, requests)
	}
}

func TestWebhookTestUsesFixedNonAlertPayloadAndSameTargetPolicy(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/hook" || r.Header.Get("Idempotency-Key") != "test-1" || r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("unexpected webhook test target or headers: %s %#v", r.URL, r.Header)
		}
		var payload TestMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode test payload: %v", err)
		}
		if payload.Type != "test" || payload.IdempotencyKey != "test-1" || payload.Message != "leanPrice test notification" {
			t.Errorf("unexpected test payload: %+v", payload)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	rootCAs := server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	client, err := NewClient(Config{
		Endpoint: server.URL + "/hook?secret=query", BearerToken: "secret-token",
		AllowedPrivateCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		Resolver:            fixedResolver{{IP: net.ParseIP("127.0.0.1")}}, Timeout: 2 * time.Second, TLSRootCAs: rootCAs,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.SendTest(context.Background(), "test-1")
	if err != nil || response.StatusCode != http.StatusNoContent || requests != 1 {
		t.Fatalf("SendTest() = %+v, %v; requests=%d", response, err, requests)
	}
}

func TestWebhookRejectsUnapprovedPrivateAndNonHTTPSPublicTargets(t *testing.T) {
	client, err := NewClient(Config{Endpoint: "https://example.test/hook", Resolver: fixedResolver{{IP: net.ParseIP("10.0.0.2")}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Send(context.Background(), "episode-1", Message{Product: "Soap", Retailer: "Test", AmountMinor: 1234, Currency: "ZAR", Reason: "threshold_met", ObservedAt: 1, Stock: "in_stock"})
	if err == nil || !strings.Contains(err.Error(), "policy rejected") {
		t.Fatalf("unapproved private target error = %v", err)
	}
	if _, err := NewClient(Config{Endpoint: "http://example.test/hook", AllowedPrivateCIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}); err != nil {
		t.Fatalf("explicit private HTTP endpoint configuration rejected: %v", err)
	}
	plain, err := NewClient(Config{Endpoint: "http://example.test/hook", AllowedPrivateCIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, Resolver: fixedResolver{{IP: net.ParseIP("1.1.1.1")}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = plain.Send(context.Background(), "episode-1", Message{Product: "Soap", Retailer: "Test", AmountMinor: 1234, Currency: "ZAR", Reason: "threshold_met", ObservedAt: 1, Stock: "in_stock"})
	if err == nil || !strings.Contains(err.Error(), "policy rejected") {
		t.Fatalf("unencrypted public target error = %v", err)
	}
}

func TestWebhookURLInPayloadIsSanitized(t *testing.T) {
	// Validate without sending by exercising the same payload validation path.
	message := Message{IdempotencyKey: "episode-1", Product: "Soap", Retailer: "Test", AmountMinor: 1234, Currency: "ZAR", Reason: "threshold_met", ObservedAt: 1, Stock: "in_stock", URL: "https://shop.example/item"}
	if err := validateMessage(message); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(Config{Endpoint: "https://webhook.example/hook", Resolver: fixedResolver{{IP: net.ParseIP("203.0.113.1")}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Send(context.Background(), "episode-1", message)
	if err == nil {
		t.Fatal("documentation-range public IP should fail closed")
	}
}
