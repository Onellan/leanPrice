package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("storage: record not found")
var ErrLeaseLost = errors.New("storage: job lease is no longer owned")
var ErrQueueFull = errors.New("storage: bounded job queue is full")

const MaxPendingJobs = 2000
const MaxJobAttempts = 5

// ScheduleRefreshes adds bounded daily jobs only for exact listings attached
// to active watches and currently approved, enabled sources.
func (s *Store) ScheduleRefreshes(ctx context.Context, now time.Time, interval time.Duration, limit int) (int, error) {
	if now.IsZero() || interval < time.Hour || interval > 30*24*time.Hour {
		return 0, errors.New("storage: schedule interval must be between 1h and 30d")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT l.id,l.source_id,l.last_checked_at
		FROM watches w JOIN listings l ON l.canonical_product_id=w.canonical_product_id
		JOIN sources s ON s.id=l.source_id
		WHERE w.active=1 AND (w.listing_id IS NULL OR w.listing_id=l.id) AND l.match_state='exact'
		AND s.assessment_state='approved' AND s.enabled=1
		AND (l.last_checked_at IS NULL OR l.last_checked_at<=?)
		ORDER BY COALESCE(l.last_checked_at,0),l.id LIMIT ?`, now.Add(-interval).UTC().UnixMilli(), limit)
	if err != nil {
		return 0, fmt.Errorf("storage: select due listing refreshes: %w", err)
	}
	type dueListing struct{ listingID, sourceID string }
	due := make([]dueListing, 0, limit)
	for rows.Next() {
		var item dueListing
		var lastChecked sql.NullInt64
		if err := rows.Scan(&item.listingID, &item.sourceID, &lastChecked); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("storage: scan due listing refresh: %w", err)
		}
		due = append(due, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("storage: read due listing refreshes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("storage: close due listing refreshes: %w", err)
	}
	bucket := now.UTC().Format("20060102")
	created := 0
	for _, item := range due {
		hash := sha256.Sum256([]byte(item.listingID))
		jitter := time.Duration((uint16(hash[0])<<8|uint16(hash[1]))%uint16((3*time.Hour)/time.Second)) * time.Second
		job := Job{ID: fmt.Sprintf("refresh-%x-%s", hash[:8], bucket), Kind: "refresh_listing", SourceID: item.sourceID,
			ListingID: item.listingID, DedupeKey: "refresh:" + item.listingID + ":" + bucket,
			DueAt: now.Add(jitter).UTC().UnixMilli(), Status: "queued"}
		_, wasCreated, err := s.EnqueueJob(ctx, job)
		if err != nil && !errors.Is(err, ErrQueueFull) {
			return created, err
		}
		if wasCreated {
			created++
		}
	}
	return created, nil
}

// Job is a durable unit of scheduled work. Source-backed work can only be
// claimed when its source remains approved and enabled.
type Job struct {
	ID            string
	Kind          string
	SourceID      string
	ListingID     string
	DedupeKey     string
	Status        string
	AttemptCount  int
	DueAt         int64
	LeaseUntil    int64
	LeasedBy      string
	LastErrorCode string
	CreatedAt     int64
	UpdatedAt     int64
}

// EnqueueJob inserts a job once per dedupe key. It returns the durable job ID
// and whether this call created it.
func (s *Store) EnqueueJob(ctx context.Context, job Job) (id string, created bool, err error) {
	if err := validateNewJob(job); err != nil {
		return "", false, err
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if job.Status != "queued" {
		return "", false, errors.New("storage: new jobs must start queued")
	}
	if job.CreatedAt == 0 {
		job.CreatedAt = time.Now().UTC().UnixMilli()
	}
	if job.UpdatedAt == 0 {
		job.UpdatedAt = job.CreatedAt
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("storage: begin job enqueue: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := tx.QueryRowContext(ctx, "SELECT id FROM jobs WHERE dedupe_key = ?", job.DedupeKey).Scan(&id); err == nil {
		if err := tx.Commit(); err != nil {
			return "", false, fmt.Errorf("storage: commit deduplicated job: %w", err)
		}
		return id, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("storage: check job dedupe key: %w", err)
	}
	var pending int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM jobs WHERE status IN ('queued','leased','retry_at')`).Scan(&pending); err != nil {
		return "", false, fmt.Errorf("storage: count pending jobs: %w", err)
	}
	if pending >= MaxPendingJobs {
		return "", false, ErrQueueFull
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO jobs
		(id, job_kind, source_id, listing_id, dedupe_key, status, attempt_count, due_at, created_at, updated_at)
		VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, 'queued', 0, ?, ?, ?)`,
		job.ID, job.Kind, job.SourceID, job.ListingID, job.DedupeKey, job.DueAt, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return "", false, fmt.Errorf("storage: enqueue job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("storage: commit job enqueue: %w", err)
	}
	return job.ID, true, nil
}

func validateNewJob(job Job) error {
	if strings.TrimSpace(job.ID) == "" || strings.TrimSpace(job.DedupeKey) == "" {
		return errors.New("storage: job ID and dedupe key are required")
	}
	switch job.Kind {
	case "refresh_listing", "source_probe", "notification_delivery", "maintenance":
	default:
		return fmt.Errorf("storage: unsupported job kind %q", job.Kind)
	}
	if job.DueAt < 0 {
		return errors.New("storage: due time cannot be negative")
	}
	return nil
}

// ClaimJob atomically leases one due job. The source approval predicate is
// checked in the same SQL statement that claims source-backed work.
func (s *Store) ClaimJob(ctx context.Context, worker string, now time.Time, lease time.Duration) (Job, error) {
	worker = strings.TrimSpace(worker)
	if worker == "" || strings.ContainsRune(worker, '\x00') {
		return Job{}, errors.New("storage: a worker identity is required")
	}
	if lease < time.Millisecond || lease > 24*time.Hour {
		return Job{}, errors.New("storage: lease duration must be between 1ms and 24h")
	}
	nowMS := now.UTC().UnixMilli()
	leaseUntil := now.Add(lease).UTC().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, fmt.Errorf("storage: begin job claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT j.id FROM jobs j
		WHERE ((j.status IN ('queued','retry_at') AND j.due_at <= ?) OR
		       (j.status = 'leased' AND j.lease_until <= ?))
		  AND (j.source_id IS NULL OR EXISTS (
		      SELECT 1 FROM sources s WHERE s.id = j.source_id
		      AND s.assessment_state = 'approved' AND s.enabled = 1))
		ORDER BY j.due_at, j.created_at, j.id LIMIT 1`, nowMS, nowMS).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("storage: select due job: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE jobs SET status = 'leased', attempt_count = attempt_count + 1,
		lease_until = ?, leased_by = ?, updated_at = ?
		WHERE id = ? AND ((status IN ('queued','retry_at') AND due_at <= ?) OR
		                  (status = 'leased' AND lease_until <= ?))
		  AND (source_id IS NULL OR EXISTS (SELECT 1 FROM sources s WHERE s.id = jobs.source_id
		      AND s.assessment_state = 'approved' AND s.enabled = 1))`, leaseUntil, worker, nowMS, id, nowMS, nowMS)
	if err != nil {
		return Job{}, fmt.Errorf("storage: lease job: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return Job{}, ErrNotFound
	}
	job, err := scanJob(tx.QueryRowContext(ctx, `SELECT id, job_kind, COALESCE(source_id,''), COALESCE(listing_id,''),
		dedupe_key, status, attempt_count, due_at, lease_until, leased_by, COALESCE(last_error_code,''), created_at, updated_at
		FROM jobs WHERE id = ?`, id))
	if err != nil {
		return Job{}, fmt.Errorf("storage: read claimed job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Job{}, fmt.Errorf("storage: commit job claim: %w", err)
	}
	return job, nil
}

// JobResult is the only allowed outcome of a leased job.
type JobResult struct {
	Status        string
	NextAttemptAt int64
	LastErrorCode string
}

// FinishJob performs an owner-checked state transition and releases the lease.
func (s *Store) FinishJob(ctx context.Context, id, worker string, result JobResult, now time.Time) error {
	switch result.Status {
	case "completed", "retry_at", "blocked", "dead":
	default:
		return fmt.Errorf("storage: invalid terminal job transition %q", result.Status)
	}
	if result.Status == "retry_at" && result.NextAttemptAt <= now.UnixMilli() {
		return errors.New("storage: retry time must be in the future")
	}
	if len(result.LastErrorCode) > 80 || strings.ContainsRune(result.LastErrorCode, '\x00') {
		return errors.New("storage: invalid job error code")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin job finish: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var attempts int
	if err := tx.QueryRowContext(ctx, `SELECT attempt_count FROM jobs WHERE id = ? AND status = 'leased' AND leased_by = ?`, id, worker).Scan(&attempts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrLeaseLost
		}
		return fmt.Errorf("storage: read job lease before finish: %w", err)
	}
	status := result.Status
	if status == "retry_at" && attempts >= MaxJobAttempts {
		status = "dead"
	}
	_, err = tx.ExecContext(ctx, `UPDATE jobs SET status = ?, due_at = CASE WHEN ? = 'retry_at' THEN ? ELSE due_at END,
		lease_until = NULL, leased_by = NULL, last_error_code = NULLIF(?, ''), updated_at = ?
		WHERE id = ? AND status = 'leased' AND leased_by = ?`,
		status, status, result.NextAttemptAt, result.LastErrorCode, now.UTC().UnixMilli(), id, worker)
	if err != nil {
		return fmt.Errorf("storage: finish job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit job finish: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var leaseUntil sql.NullInt64
	var leasedBy sql.NullString
	err := row.Scan(&job.ID, &job.Kind, &job.SourceID, &job.ListingID, &job.DedupeKey, &job.Status,
		&job.AttemptCount, &job.DueAt, &leaseUntil, &leasedBy, &job.LastErrorCode, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return Job{}, err
	}
	if leaseUntil.Valid {
		job.LeaseUntil = leaseUntil.Int64
	}
	if leasedBy.Valid {
		job.LeasedBy = leasedBy.String
	}
	return job, nil
}
