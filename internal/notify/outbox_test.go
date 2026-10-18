package notify

import (
	"context"
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/storage"
)

type fakeRepository struct {
	delivery storage.Delivery
	payload  storage.DeliveryPayload
	result   storage.DeliveryResult
	claims   int
	finishes int
}

func (r *fakeRepository) ClaimDelivery(context.Context, string, time.Time, time.Duration) (storage.Delivery, error) {
	r.claims++
	if r.claims > 1 {
		return storage.Delivery{}, storage.ErrNoDelivery
	}
	r.delivery.AttemptCount = 1
	return r.delivery, nil
}
func (r *fakeRepository) PayloadForDelivery(context.Context, string) (storage.DeliveryPayload, error) {
	return r.payload, nil
}
func (r *fakeRepository) FinishDelivery(_ context.Context, _, _ string, result storage.DeliveryResult, _ time.Time) error {
	r.finishes++
	r.result = result
	return nil
}

type fakeGate struct{ allowed bool }

func (g fakeGate) CanLease(context.Context) (bool, string, error) {
	return g.allowed, "memory_pressure", nil
}

type fakeSender struct {
	response Response
	err      error
	key      string
	message  Message
}

func (s *fakeSender) Send(_ context.Context, key string, message Message) (Response, error) {
	s.key, s.message = key, message
	return s.response, s.err
}

func TestDeliveryServicePersistsSuccessAndRetryWithoutSensitiveErrors(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		response Response
		status   string
		delay    time.Duration
	}{
		{name: "success", response: Response{StatusCode: 204}, status: "delivered"},
		{name: "rate limited", response: Response{StatusCode: 429, RetryAfter: 30 * time.Second}, status: "retry_at", delay: 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{delivery: storage.Delivery{ID: "delivery-1", IdempotencyKey: "episode-1"}, payload: storage.DeliveryPayload{Product: "Soap", Retailer: "Example", AmountMinor: 1234, Currency: "ZAR", ReasonCode: "threshold_met", ObservedAt: now.UnixMilli(), Stock: "unknown", URL: "https://shop.example/item?tracking=secret"}}
			sender := &fakeSender{response: tc.response}
			service := DeliveryService{Repository: repo, Sender: sender, Gate: fakeGate{allowed: true}, WorkerID: "worker", Now: func() time.Time { return now }}
			worked, err := service.RunOnce(context.Background())
			if err != nil || !worked || repo.finishes != 1 || repo.result.Status != tc.status {
				t.Fatalf("RunOnce() = %v, %v; finish=%d result=%+v", worked, err, repo.finishes, repo.result)
			}
			if tc.delay > 0 && repo.result.NextAttemptAt != now.Add(tc.delay).UnixMilli() {
				t.Fatalf("retry time = %d, want %s", repo.result.NextAttemptAt, now.Add(tc.delay))
			}
			if sender.message.Stock != "unknown" || sender.message.URL != "https://shop.example/item" || sender.key != "episode-1" {
				t.Fatalf("notification payload lost its qualifier or sanitized URL: %+v", sender.message)
			}
			if repo.result.LastErrorCode == "secret details must never persist" {
				t.Fatal("raw destination error was persisted")
			}
		})
	}
}

func TestDeliveryServiceDefersBeforeClaim(t *testing.T) {
	repo := &fakeRepository{}
	service := DeliveryService{Repository: repo, Sender: &fakeSender{}, Gate: fakeGate{}, WorkerID: "worker"}
	worked, err := service.RunOnce(context.Background())
	if err != nil || worked || repo.claims != 0 {
		t.Fatalf("RunOnce() = %v, %v; claims=%d", worked, err, repo.claims)
	}
}
