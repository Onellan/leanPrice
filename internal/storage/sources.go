package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ReserveHost atomically reserves the next source request slot. Waiting time
// survives restarts through sources.next_allowed_at.
func (s *Store) ReserveHost(ctx context.Context, host string, now time.Time, minimumInterval time.Duration) (time.Duration, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || now.IsZero() || minimumInterval < 10*time.Second || minimumInterval > 24*time.Hour {
		return 0, errors.New("storage: invalid source rate reservation")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("storage: begin source rate reservation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var sourceID string
	var sourceInterval, nextAllowed int64
	err = tx.QueryRowContext(ctx, `SELECT id, min_interval_ms, next_allowed_at FROM sources
		WHERE lower(rtrim(base_host, '.')) = ? AND assessment_state = 'approved' AND enabled = 1`, host).
		Scan(&sourceID, &sourceInterval, &nextAllowed)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrSourceUnavailable
	}
	if err != nil {
		return 0, fmt.Errorf("storage: read source rate state: %w", err)
	}
	nowMS := now.UTC().UnixMilli()
	if nowMS <= 0 || sourceInterval < (10*time.Second).Milliseconds() || sourceInterval > (365*24*time.Hour).Milliseconds() {
		return 0, errors.New("storage: source rate state is outside supported bounds")
	}
	intervalMS := minimumInterval.Milliseconds()
	if sourceInterval > intervalMS {
		intervalMS = sourceInterval
	}
	startMS := nowMS
	if nextAllowed > startMS {
		startMS = nextAllowed
	}
	if startMS > int64(^uint64(0)>>1)-intervalMS || startMS-nowMS > (365*24*time.Hour).Milliseconds() {
		return 0, errors.New("storage: source rate deadline exceeds supported bounds")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sources SET next_allowed_at = ?, updated_at = ? WHERE id = ?`, startMS+intervalMS, nowMS, sourceID); err != nil {
		return 0, fmt.Errorf("storage: reserve source request slot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("storage: commit source rate reservation: %w", err)
	}
	if startMS <= nowMS {
		return 0, nil
	}
	return time.Duration(startMS-nowMS) * time.Millisecond, nil
}

// DeferHost persists a Retry-After or backoff deadline for an approved source.
func (s *Store) DeferHost(ctx context.Context, host string, until time.Time) error {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || until.IsZero() {
		return errors.New("storage: invalid source defer deadline")
	}
	deadline := until.UTC().UnixMilli()
	nowMS := time.Now().UTC().UnixMilli()
	result, err := s.db.ExecContext(ctx, `UPDATE sources
		SET next_allowed_at = CASE WHEN next_allowed_at < ? THEN ? ELSE next_allowed_at END,
		    updated_at = CASE WHEN updated_at < ? THEN ? ELSE updated_at END
		WHERE lower(rtrim(base_host, '.')) = ? AND assessment_state = 'approved' AND enabled = 1`,
		deadline, deadline, nowMS, nowMS, host)
	if err != nil {
		return fmt.Errorf("storage: defer source requests: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return ErrSourceUnavailable
	}
	return nil
}

// HostApproved implements the fetch.HostPolicy contract. Source hosts match
// exactly; sibling or wildcard subdomains are never implied by approval.
func (s *Store) HostApproved(ctx context.Context, host string) (bool, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return false, nil
	}
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM sources
		WHERE lower(rtrim(base_host, '.')) = ?
		  AND assessment_state = 'approved' AND enabled = 1
	)`, host).Scan(&enabled)
	return enabled == 1, err
}
