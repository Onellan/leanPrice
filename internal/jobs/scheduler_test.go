package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/storage"
)

type fakeRepo struct {
	job      storage.Job
	claimed  int
	finished int
	last     storage.JobResult
}

func (r *fakeRepo) ClaimJob(context.Context, string, time.Time, time.Duration) (storage.Job, error) {
	r.claimed++
	if r.claimed > 1 {
		return storage.Job{}, storage.ErrNotFound
	}
	r.job.AttemptCount = 1
	r.job.Status = "leased"
	return r.job, nil
}

func (r *fakeRepo) FinishJob(_ context.Context, _, _ string, result storage.JobResult, _ time.Time) error {
	r.finished++
	r.last = result
	return nil
}

type fakeGate struct {
	allowed bool
	reason  string
	err     error
}

func (g fakeGate) CanLease(context.Context) (bool, string, error) { return g.allowed, g.reason, g.err }

type fakeProcessor struct{ err error }

func (p fakeProcessor) Process(context.Context, storage.Job) error { return p.err }

func TestSchedulerDefersBeforeClaim(t *testing.T) {
	repo := &fakeRepo{job: storage.Job{ID: "job-1"}}
	deferred := ""
	scheduler := Scheduler{Repository: repo, Gate: fakeGate{reason: "memory_pressure"}, Processor: fakeProcessor{}, WorkerID: "worker", OnDeferred: func(_ context.Context, reason string, _ time.Time) error { deferred = reason; return nil }}
	worked, err := scheduler.RunOnce(context.Background())
	if err != nil || worked || repo.claimed != 0 || deferred != "memory_pressure" {
		t.Fatalf("RunOnce() = %v, %v; claims=%d defer=%q", worked, err, repo.claimed, deferred)
	}
}

func TestSchedulerFinishesOneJobWithBoundedRetry(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{job: storage.Job{ID: "job-1"}}
	scheduler := Scheduler{Repository: repo, Gate: fakeGate{allowed: true}, Processor: fakeProcessor{err: errors.New("secret details must not persist")}, WorkerID: "worker", Now: func() time.Time { return now }}
	worked, err := scheduler.RunOnce(context.Background())
	if err != nil || !worked || repo.claimed != 1 || repo.finished != 1 || repo.last.Status != "retry_at" || repo.last.LastErrorCode != "processing_failed" || repo.last.NextAttemptAt != now.Add(5*time.Second).UnixMilli() {
		t.Fatalf("RunOnce() = %v, %v; claim=%d finish=%d result=%+v", worked, err, repo.claimed, repo.finished, repo.last)
	}
}
