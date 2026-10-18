package notify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Onellan/leanPrice/internal/extract"
	"github.com/Onellan/leanPrice/internal/storage"
)

type OutboxRepository interface {
	ClaimDelivery(context.Context, string, time.Time, time.Duration) (storage.Delivery, error)
	PayloadForDelivery(context.Context, string) (storage.DeliveryPayload, error)
	FinishDelivery(context.Context, string, string, storage.DeliveryResult, time.Time) error
}

type Sender interface {
	Send(context.Context, string, Message) (Response, error)
}

type DeliveryService struct {
	Repository OutboxRepository
	Sender     Sender
	Gate       interface {
		CanLease(context.Context) (bool, string, error)
	}
	WorkerID   string
	Now        func() time.Time
	OnDeferred func(context.Context, string, time.Time) error
}

func (s DeliveryService) RunOnce(ctx context.Context) (bool, error) {
	if s.Repository == nil || s.Sender == nil || s.Gate == nil || s.WorkerID == "" {
		return false, errors.New("notify: repository, sender, resource gate and worker ID are required")
	}
	now := s.now()
	allowed, reason, gateErr := s.Gate.CanLease(ctx)
	if gateErr != nil || !allowed {
		if reason == "" {
			reason = "resource_metric_unavailable"
		}
		if s.OnDeferred != nil {
			if err := s.OnDeferred(ctx, reason, now); err != nil {
				return false, fmt.Errorf("notify: record delivery deferral: %w", err)
			}
		}
		return false, gateErr
	}
	delivery, err := s.Repository.ClaimDelivery(ctx, s.WorkerID, now, 2*time.Minute)
	if errors.Is(err, storage.ErrNoDelivery) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("notify: claim outbox delivery: %w", err)
	}
	payload, err := s.Repository.PayloadForDelivery(ctx, delivery.ID)
	if err != nil {
		return true, s.finish(ctx, delivery.ID, storage.DeliveryResult{Status: "dead", LastErrorCode: "payload_unavailable"})
	}
	response, sendErr := s.Sender.Send(ctx, delivery.IdempotencyKey, Message{
		Product: payload.Product, Retailer: payload.Retailer, AmountMinor: payload.AmountMinor,
		Currency: payload.Currency, Reason: payload.ReasonCode, ObservedAt: payload.ObservedAt,
		Stock: payload.Stock, URL: extract.SanitizeURL(payload.URL),
	})
	result := storage.DeliveryResult{HTTPStatus: response.StatusCode}
	finishedAt := s.now()
	switch {
	case sendErr == nil && response.StatusCode >= 200 && response.StatusCode < 300:
		result.Status = "delivered"
	case sendErr == nil && response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != 429:
		result.Status, result.LastErrorCode = "dead", "destination_rejected"
	case sendErr == nil && response.StatusCode >= 300 && response.StatusCode < 400:
		result.Status, result.LastErrorCode = "dead", "redirect_rejected"
	default:
		result.Status, result.LastErrorCode = "retry_at", "delivery_failed"
		delay := retryDelay(delivery.AttemptCount)
		if sendErr == nil && response.StatusCode == 429 && response.RetryAfter > 0 {
			delay = response.RetryAfter
		}
		if delay > 24*time.Hour {
			delay = 24 * time.Hour
		}
		result.NextAttemptAt = finishedAt.Add(delay).UnixMilli()
	}
	return true, s.finish(ctx, delivery.ID, result)
}

func (s DeliveryService) finish(ctx context.Context, id string, result storage.DeliveryResult) error {
	if err := s.Repository.FinishDelivery(ctx, id, s.WorkerID, result, s.now()); err != nil {
		return fmt.Errorf("notify: finish outbox delivery: %w", err)
	}
	return nil
}

func (s DeliveryService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 5 * time.Second
	for n := 1; n < attempt && delay < time.Hour; n++ {
		delay *= 2
	}
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}
