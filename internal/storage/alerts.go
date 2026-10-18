package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrAlertDestinationMismatch = errors.New("storage: alert destination does not match the current watch preference")

// AlertDelivery describes the single destination inserted into the durable
// outbox alongside a new logical alert episode.
type AlertDelivery struct {
	EpisodeID      string
	WatchID        string
	RuleVersion    int
	OfferID        string
	EpisodeKey     string
	ReasonCode     string
	DestinationKey string
	IdempotencyKey string
	CreatedAt      int64
	NextAttemptAt  int64
	MaxAge         time.Duration
}

type AlertRecovery struct {
	WatchID     string
	RuleVersion int
	OfferID     string
	RecoveredAt int64
	MaxAge      time.Duration
}

// CreateAlertEpisode atomically checks offer lineage and current watch rules,
// then inserts the logical episode and one idempotent outbox row.
func (s *Store) CreateAlertEpisode(ctx context.Context, alert AlertDelivery) (episodeID string, created bool, err error) {
	if strings.TrimSpace(alert.EpisodeID) == "" || strings.TrimSpace(alert.WatchID) == "" || strings.TrimSpace(alert.OfferID) == "" ||
		strings.TrimSpace(alert.EpisodeKey) == "" || strings.TrimSpace(alert.ReasonCode) == "" || strings.TrimSpace(alert.DestinationKey) == "" ||
		strings.TrimSpace(alert.IdempotencyKey) == "" || alert.RuleVersion < 1 {
		return "", false, errors.New("storage: alert episode identity, reason, rule and destination are required")
	}
	if alert.MaxAge <= 0 || alert.MaxAge > 365*24*time.Hour {
		return "", false, errors.New("storage: alert freshness window must be between 1ns and 365d")
	}
	if alert.CreatedAt == 0 {
		alert.CreatedAt = time.Now().UTC().UnixMilli()
	}
	if alert.NextAttemptAt == 0 {
		alert.NextAttemptAt = alert.CreatedAt
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("storage: begin alert transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var listingID, notificationChannel string
	err = tx.QueryRowContext(ctx, `SELECT l.id,COALESCE(rv.notification_channel,'')
		FROM watches w
		JOIN watch_rule_versions rv ON rv.watch_id = w.id AND rv.version = w.current_rule_version
		JOIN observed_offers o ON o.id = ?
		JOIN observations ob ON ob.id = o.observation_id
		JOIN listings l ON l.id = ob.listing_id AND l.canonical_product_id = w.canonical_product_id
		JOIN sources s ON s.id = l.source_id
		WHERE w.id = ? AND w.active = 1 AND w.current_rule_version = ? AND rv.version = ?
		  AND ob.fetch_status = 'success' AND ob.validation_state = 'valid'
		  AND ob.observed_at >= ? AND ob.observed_at <= ?
		  AND l.match_state = 'exact'
		  AND (w.listing_id IS NULL OR w.listing_id = l.id)
		  AND s.enabled = 1 AND s.assessment_state = 'approved'
		  AND o.currency = 'ZAR' AND o.amount_minor > 0 AND o.offer_kind != 'coupon'
		  AND (o.eligibility_state = 'public' OR (rv.allow_member = 1 AND o.eligibility_state = 'conditional'))
		  AND (o.offer_kind != 'multibuy' OR rv.allow_multibuy = 1)
		AND (o.valid_from IS NULL OR o.valid_from <= ?) AND (o.valid_until IS NULL OR o.valid_until > ?)
		AND ob.observed_stock != 'out_of_stock'`,
		alert.OfferID, alert.WatchID, alert.RuleVersion, alert.RuleVersion,
		alert.CreatedAt-alert.MaxAge.Milliseconds(), alert.CreatedAt, alert.CreatedAt, alert.CreatedAt).Scan(&listingID, &notificationChannel)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, errors.New("storage: alert offer is stale, ineligible, or outside the watch scope")
	}
	if err != nil {
		return "", false, fmt.Errorf("storage: validate alert lineage: %w", err)
	}
	if notificationChannel != "" && notificationChannel != alert.DestinationKey {
		return "", false, ErrAlertDestinationMismatch
	}
	var openEpisodeID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM alert_episodes
		WHERE watch_id = ? AND rule_version = ? AND listing_id = ? AND closed_at IS NULL`,
		alert.WatchID, alert.RuleVersion, listingID).Scan(&openEpisodeID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return "", false, fmt.Errorf("storage: commit duplicate alert check: %w", err)
		}
		return openEpisodeID, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("storage: read open alert episode: %w", err)
	}
	var episodeNumber int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(episode_number), 0) + 1 FROM alert_episodes
		WHERE watch_id = ? AND rule_version = ? AND listing_id = ?`, alert.WatchID, alert.RuleVersion, listingID).Scan(&episodeNumber); err != nil {
		return "", false, fmt.Errorf("storage: allocate alert episode number: %w", err)
	}
	episodeKey := fmt.Sprintf("%s:cycle:%d", alert.EpisodeKey, episodeNumber)
	if len(episodeKey) > 240 {
		return "", false, errors.New("storage: alert episode key exceeds 240 bytes")
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO alert_episodes
		(id, watch_id, rule_version, listing_id, episode_number, observed_offer_id, episode_key, reason_code, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(episode_key) DO NOTHING`,
		alert.EpisodeID, alert.WatchID, alert.RuleVersion, listingID, episodeNumber, alert.OfferID, episodeKey, alert.ReasonCode, alert.CreatedAt)
	if err != nil {
		return "", false, fmt.Errorf("storage: insert alert episode: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", false, fmt.Errorf("storage: alert insert result: %w", err)
	}
	created = rows == 1
	if created {
		episodeID = alert.EpisodeID
	} else {
		var watchID, offerID string
		var rule int
		if err := tx.QueryRowContext(ctx, "SELECT id, watch_id, rule_version, observed_offer_id FROM alert_episodes WHERE episode_key = ?", episodeKey).Scan(&episodeID, &watchID, &rule, &offerID); err != nil {
			return "", false, fmt.Errorf("storage: find existing alert episode: %w", err)
		}
		if watchID != alert.WatchID || rule != alert.RuleVersion || offerID != alert.OfferID {
			return "", false, errors.New("storage: episode key is already bound to different alert evidence")
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO delivery_outbox
		(id, alert_episode_id, destination_key, idempotency_key, status, next_attempt_at)
		VALUES (?, ?, ?, ?, 'pending', ?) ON CONFLICT(alert_episode_id, destination_key) DO NOTHING`,
		alert.EpisodeID+"-delivery", episodeID, alert.DestinationKey, alert.IdempotencyKey, alert.NextAttemptAt)
	if err != nil {
		return "", false, fmt.Errorf("storage: enqueue alert delivery: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("storage: commit alert episode: %w", err)
	}
	return episodeID, created, nil
}

// CloseRecoveredAlertEpisode closes an open episode after the caller has
// evaluated a fresh exact offer and confirmed that the configured rule has
// recovered. It checks the recovery offer's current lineage in the transaction.
func (s *Store) CloseRecoveredAlertEpisode(ctx context.Context, recovery AlertRecovery) (bool, error) {
	if strings.TrimSpace(recovery.WatchID) == "" || strings.TrimSpace(recovery.OfferID) == "" || recovery.RuleVersion < 1 || recovery.RecoveredAt <= 0 || recovery.MaxAge <= 0 || recovery.MaxAge > 365*24*time.Hour {
		return false, errors.New("storage: invalid alert recovery evidence")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("storage: begin alert recovery: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var listingID string
	err = tx.QueryRowContext(ctx, `SELECT l.id
		FROM watches w
		JOIN watch_rule_versions rv ON rv.watch_id = w.id AND rv.version = w.current_rule_version
		JOIN observed_offers o ON o.id = ?
		JOIN observations ob ON ob.id = o.observation_id
		JOIN listings l ON l.id = ob.listing_id AND l.canonical_product_id = w.canonical_product_id
		JOIN sources s ON s.id = l.source_id
		WHERE w.id = ? AND w.active = 1 AND w.current_rule_version = ? AND rv.version = ?
		  AND ob.fetch_status = 'success' AND ob.validation_state = 'valid'
		  AND ob.observed_at >= ? AND ob.observed_at <= ? AND l.match_state = 'exact'
		  AND (w.listing_id IS NULL OR w.listing_id = l.id)
		  AND s.enabled = 1 AND s.assessment_state = 'approved'
		  AND o.currency = 'ZAR' AND o.amount_minor > 0 AND o.offer_kind != 'coupon'
		  AND (o.eligibility_state = 'public' OR (rv.allow_member = 1 AND o.eligibility_state = 'conditional'))
		  AND (o.offer_kind != 'multibuy' OR rv.allow_multibuy = 1)
		  AND (o.valid_from IS NULL OR o.valid_from <= ?) AND (o.valid_until IS NULL OR o.valid_until > ?)
		  AND ob.observed_stock != 'out_of_stock'`, recovery.OfferID, recovery.WatchID, recovery.RuleVersion, recovery.RuleVersion,
		recovery.RecoveredAt-recovery.MaxAge.Milliseconds(), recovery.RecoveredAt, recovery.RecoveredAt, recovery.RecoveredAt).Scan(&listingID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, errors.New("storage: alert recovery offer is stale, ineligible, or outside the watch scope")
	}
	if err != nil {
		return false, fmt.Errorf("storage: validate alert recovery lineage: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE alert_episodes SET closed_at = ?
		WHERE watch_id = ? AND rule_version = ? AND listing_id = ? AND closed_at IS NULL`,
		recovery.RecoveredAt, recovery.WatchID, recovery.RuleVersion, listingID)
	if err != nil {
		return false, fmt.Errorf("storage: close recovered alert episode: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: read recovered episode result: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("storage: commit alert recovery: %w", err)
	}
	return count > 0, nil
}
