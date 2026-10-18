package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fixedReader struct {
	bytes uint64
	err   error
}

func (r fixedReader) CurrentBytes(context.Context) (uint64, error) { return r.bytes, r.err }

func TestMemoryGovernorHysteresis(t *testing.T) {
	governor := MemoryGovernor{Reader: fixedReader{bytes: 360}, SoftLimit: 350, ResumeAt: 320}
	if allowed, reason, err := governor.CanLease(context.Background()); err != nil || allowed || reason != "memory_pressure" {
		t.Fatalf("high memory gate = %v, %q, %v", allowed, reason, err)
	}
	status, err := governor.Status(context.Background())
	if err != nil || status.CurrentBytes != 360 || status.SoftLimit != 350 || status.ResumeAt != 320 || !status.Paused {
		t.Fatalf("memory status = %+v, %v", status, err)
	}
	governor.Reader = fixedReader{bytes: 330}
	if allowed, _, err := governor.CanLease(context.Background()); err != nil || allowed {
		t.Fatalf("hysteresis gate resumed too early: %v, %v", allowed, err)
	}
	governor.Reader = fixedReader{bytes: 319}
	if allowed, reason, err := governor.CanLease(context.Background()); err != nil || !allowed || reason != "" {
		t.Fatalf("low memory gate = %v, %q, %v", allowed, reason, err)
	}
}

func TestCgroupReaderIsBoundedAndFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.current")
	if err := os.WriteFile(path, []byte("123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (CgroupV2Reader{Path: path}).CurrentBytes(context.Background())
	if err != nil || got != 123456 {
		t.Fatalf("CurrentBytes() = %d, %v", got, err)
	}
	if _, err := (CgroupV2Reader{Path: filepath.Join(t.TempDir(), "missing")}).CurrentBytes(context.Background()); err == nil {
		t.Fatal("missing cgroup metric must fail closed")
	}
	governor := MemoryGovernor{Reader: fixedReader{err: errors.New("unavailable")}, SoftLimit: 350, ResumeAt: 320}
	if allowed, reason, err := governor.CanLease(context.Background()); err == nil || allowed || reason != "memory_metric_unavailable" {
		t.Fatalf("metric failure gate = %v, %q, %v", allowed, reason, err)
	}
}
