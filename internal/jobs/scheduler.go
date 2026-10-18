// Package jobs runs durable work with one bounded worker and an explicit
// resource gate. It never creates a goroutine per job.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Onellan/leanPrice/internal/storage"
)

var ErrNoJob = storage.ErrNotFound

type Repository interface {
	ClaimJob(context.Context, string, time.Time, time.Duration) (storage.Job, error)
	FinishJob(context.Context, string, string, storage.JobResult, time.Time) error
}

type ResourceGate interface {
	CanLease(context.Context) (bool, string, error)
}

type Processor interface {
	Process(context.Context, storage.Job) error
}

type Failure struct {
	Status     string
	Code       string
	RetryAfter time.Duration
	Cause      error
}

func (f *Failure) Error() string {
	if f == nil || f.Cause == nil {
		return "jobs: processing failed"
	}
	return f.Cause.Error()
}

func (f *Failure) Unwrap() error { return f.Cause }

type DeferredEvent func(context.Context, string, time.Time) error

type Scheduler struct {
	Repository   Repository
	Gate         ResourceGate
	Processor    Processor
	BeforeRun    func(context.Context, time.Time) error
	WorkerID     string
	Lease        time.Duration
	PollInterval time.Duration
	Now          func() time.Time
	OnDeferred   DeferredEvent
}

// RunOnce checks resource pressure before claiming durable work. An empty
// queue is a normal no-op. Errors returned by Process become bounded retries.
func (s Scheduler) RunOnce(ctx context.Context) (bool, error) {
	if s.Repository == nil || s.Gate == nil || s.Processor == nil || s.WorkerID == "" {
		return false, errors.New("jobs: repository, resource gate, processor and worker ID are required")
	}
	now := s.now()
	allowed, reason, err := s.Gate.CanLease(ctx)
	if err != nil || !allowed {
		if reason == "" {
			reason = "resource_metric_unavailable"
		}
		if s.OnDeferred != nil {
			if recordErr := s.OnDeferred(ctx, reason, now); recordErr != nil {
				return false, fmt.Errorf("jobs: record resource deferral: %w", recordErr)
			}
		}
		return false, nil // fail closed, and let the managed poller recheck the metric
	}
	if s.BeforeRun != nil {
		if err := s.BeforeRun(ctx, now); err != nil {
			return false, fmt.Errorf("jobs: enqueue scheduled work: %w", err)
		}
	}
	lease := s.Lease
	if lease <= 0 || lease > 24*time.Hour {
		lease = 2 * time.Minute
	}
	job, err := s.Repository.ClaimJob(ctx, s.WorkerID, now, lease)
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("jobs: claim due work: %w", err)
	}
	processErr := s.Processor.Process(ctx, job)
	completedAt := s.now()
	result := storage.JobResult{Status: "completed"}
	if processErr != nil {
		var failure *Failure
		if errors.As(processErr, &failure) && (failure.Status == "blocked" || failure.Status == "dead") {
			result.Status = failure.Status
			result.LastErrorCode = failure.Code
		} else {
			result.Status = "retry_at"
			delay := retryDelay(job.AttemptCount)
			if errors.As(processErr, &failure) && failure.RetryAfter > 0 {
				delay = failure.RetryAfter
			}
			if delay > 24*time.Hour {
				delay = 24 * time.Hour
			}
			result.NextAttemptAt = completedAt.Add(delay).UnixMilli()
			result.LastErrorCode = "processing_failed"
			if errors.As(processErr, &failure) && failure.Code != "" {
				result.LastErrorCode = failure.Code
			}
		}
	}
	if finishErr := s.Repository.FinishJob(ctx, job.ID, s.WorkerID, result, completedAt); finishErr != nil {
		return true, fmt.Errorf("jobs: finish leased work: %w", finishErr)
	}
	return true, nil
}

// Start polls with one goroutine owned by the caller. Each tick handles at
// most one job; shutdown cancellation is passed through to the processor.
func (s Scheduler) Start(ctx context.Context) error {
	interval := s.PollInterval
	if interval < 100*time.Millisecond || interval > time.Minute {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s Scheduler) now() time.Time {
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
