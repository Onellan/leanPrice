package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNoDelivery = errors.New("storage: no due delivery")
var ErrDeliveryLeaseLost = errors.New("storage: delivery lease is no longer owned")

// Delivery is a leased notification intent; destination secrets are resolved
// outside the database and are never stored in this row.
type Delivery struct {
	ID             string
	EpisodeID      string
	DestinationKey string
	IdempotencyKey string
	Status         string
	AttemptCount   int
	NextAttemptAt  int64
	LeaseUntil     int64
	LeasedBy       string
	LastStatusCode int
	LastErrorCode  string
	DeliveredAt    int64
}

type DeliveryPayload struct {
	Product     string
	Retailer    string
	AmountMinor int64
	Currency    string
	ReasonCode  string
	ObservedAt  int64
	Stock       string
	URL         string
}

func (s *Store) PayloadForDelivery(ctx context.Context, deliveryID string) (DeliveryPayload, error) {
	var payload DeliveryPayload
	err := s.db.QueryRowContext(ctx, `SELECT p.display_name,r.name,o.amount_minor,o.currency,a.reason_code,ob.observed_at,
		ob.observed_stock,l.url FROM delivery_outbox d JOIN alert_episodes a ON a.id=d.alert_episode_id
		JOIN observed_offers o ON o.id=a.observed_offer_id JOIN observations ob ON ob.id=o.observation_id
		JOIN listings l ON l.id=ob.listing_id JOIN canonical_products p ON p.id=l.canonical_product_id
		JOIN sources s ON s.id=l.source_id JOIN retailers r ON r.id=s.retailer_id
		WHERE d.id=? AND d.status='leased'`, deliveryID).Scan(&payload.Product, &payload.Retailer, &payload.AmountMinor,
		&payload.Currency, &payload.ReasonCode, &payload.ObservedAt, &payload.Stock, &payload.URL)
	if errors.Is(err, sql.ErrNoRows) {
		return DeliveryPayload{}, ErrNotFound
	}
	if err != nil {
		return DeliveryPayload{}, fmt.Errorf("storage: read delivery payload: %w", err)
	}
	return payload, nil
}

// ClaimDelivery leases one due outbox row. A send must happen after this method
// returns and before FinishDelivery begins another database transaction.
func (s *Store) ClaimDelivery(ctx context.Context, worker string, now time.Time, lease time.Duration) (Delivery, error) {
	worker = strings.TrimSpace(worker)
	if worker == "" || lease < time.Millisecond || lease > 24*time.Hour {
		return Delivery{}, errors.New("storage: valid worker and 1ms–24h lease required")
	}
	nowMS := now.UTC().UnixMilli()
	leaseUntil := now.Add(lease).UTC().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Delivery{}, fmt.Errorf("storage: begin delivery claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM delivery_outbox
		WHERE destination_key='webhook' AND ((status IN ('pending','retry_at') AND next_attempt_at <= ?) OR
		      (status = 'leased' AND lease_until <= ?))
		ORDER BY next_attempt_at, id LIMIT 1`, nowMS, nowMS).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Delivery{}, ErrNoDelivery
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("storage: select due delivery: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE delivery_outbox SET status='leased', attempt_count=attempt_count+1,
		lease_until=?, leased_by=? WHERE id=? AND
		((status IN ('pending','retry_at') AND next_attempt_at <= ?) OR (status='leased' AND lease_until <= ?))`,
		leaseUntil, worker, id, nowMS, nowMS)
	if err != nil {
		return Delivery{}, fmt.Errorf("storage: lease delivery: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return Delivery{}, ErrNoDelivery
	}
	delivery, err := scanDelivery(tx.QueryRowContext(ctx, `SELECT id, alert_episode_id, destination_key, idempotency_key,
		status, attempt_count, next_attempt_at, lease_until, leased_by, last_status_code,
		COALESCE(last_error_code,''), delivered_at FROM delivery_outbox WHERE id=?`, id))
	if err != nil {
		return Delivery{}, fmt.Errorf("storage: read claimed delivery: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Delivery{}, fmt.Errorf("storage: commit delivery lease: %w", err)
	}
	return delivery, nil
}

type DeliveryResult struct {
	Status        string
	NextAttemptAt int64
	HTTPStatus    int
	LastErrorCode string
}

// FinishDelivery records a provider receipt or bounded retry decision after
// the network request has completed.
func (s *Store) FinishDelivery(ctx context.Context, id, worker string, result DeliveryResult, now time.Time) error {
	switch result.Status {
	case "delivered", "retry_at", "dead":
	default:
		return fmt.Errorf("storage: invalid delivery transition %q", result.Status)
	}
	if result.Status == "retry_at" && result.NextAttemptAt <= now.UnixMilli() {
		return errors.New("storage: delivery retry time must be in the future")
	}
	var attemptCount int
	if err := s.db.QueryRowContext(ctx, "SELECT attempt_count FROM delivery_outbox WHERE id=? AND status='leased' AND leased_by=?", id, worker).Scan(&attemptCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeliveryLeaseLost
		}
		return fmt.Errorf("storage: read delivery attempt count: %w", err)
	}
	if result.Status == "retry_at" && attemptCount >= 5 {
		result.Status = "dead"
	}
	if result.Status == "delivered" && (result.HTTPStatus < 200 || result.HTTPStatus >= 300) {
		return errors.New("storage: delivered requires a successful 2xx HTTP response")
	}
	if result.HTTPStatus < 0 || result.HTTPStatus > 599 || result.HTTPStatus > 0 && result.HTTPStatus < 100 {
		return errors.New("storage: invalid delivery HTTP status")
	}
	if len(result.LastErrorCode) > 80 || strings.ContainsRune(result.LastErrorCode, '\x00') {
		return errors.New("storage: invalid delivery error code")
	}
	deliveredAt := any(nil)
	if result.Status == "delivered" {
		deliveredAt = now.UTC().UnixMilli()
	}
	var httpStatus any
	if result.HTTPStatus != 0 {
		httpStatus = result.HTTPStatus
	}
	update, err := s.db.ExecContext(ctx, `UPDATE delivery_outbox SET status=?,
		next_attempt_at=CASE WHEN ?='retry_at' THEN ? ELSE next_attempt_at END,
		lease_until=NULL, leased_by=NULL, last_status_code=?, last_error_code=NULLIF(?, ''), delivered_at=?
		WHERE id=? AND status='leased' AND leased_by=?`, result.Status, result.Status, result.NextAttemptAt,
		httpStatus, result.LastErrorCode, deliveredAt, id, worker)
	if err != nil {
		return fmt.Errorf("storage: finish delivery: %w", err)
	}
	count, err := update.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: finish delivery result: %w", err)
	}
	if count != 1 {
		return ErrDeliveryLeaseLost
	}
	return nil
}

func scanDelivery(row rowScanner) (Delivery, error) {
	var delivery Delivery
	var leaseUntil, statusCode, deliveredAt sql.NullInt64
	var leasedBy sql.NullString
	err := row.Scan(&delivery.ID, &delivery.EpisodeID, &delivery.DestinationKey, &delivery.IdempotencyKey,
		&delivery.Status, &delivery.AttemptCount, &delivery.NextAttemptAt, &leaseUntil, &leasedBy,
		&statusCode, &delivery.LastErrorCode, &deliveredAt)
	if err != nil {
		return Delivery{}, err
	}
	if leaseUntil.Valid {
		delivery.LeaseUntil = leaseUntil.Int64
	}
	if leasedBy.Valid {
		delivery.LeasedBy = leasedBy.String
	}
	if statusCode.Valid {
		delivery.LastStatusCode = int(statusCode.Int64)
	}
	if deliveredAt.Valid {
		delivery.DeliveredAt = deliveredAt.Int64
	}
	return delivery, nil
}
