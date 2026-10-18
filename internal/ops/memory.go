// Package ops contains bounded operational controls for the local runtime.
package ops

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

type MemoryReader interface {
	CurrentBytes(context.Context) (uint64, error)
}

type CgroupV2Reader struct {
	Path string
}

func (r CgroupV2Reader) CurrentBytes(ctx context.Context) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if strings.TrimSpace(r.Path) == "" {
		return 0, errors.New("cgroup memory.current path is required")
	}
	file, err := os.Open(r.Path)
	if err != nil {
		return 0, fmt.Errorf("open cgroup memory.current: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 64))
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return 0, fmt.Errorf("read cgroup memory.current: %w", err)
		}
		return 0, errors.New("cgroup memory.current is empty")
	}
	value := strings.TrimSpace(scanner.Text())
	bytes, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cgroup memory.current: %w", err)
	}
	return bytes, nil
}

type MemoryGovernor struct {
	Reader    MemoryReader
	SoftLimit uint64
	ResumeAt  uint64
	mu        sync.Mutex
	paused    bool
}

type MemoryStatus struct {
	CurrentBytes uint64
	SoftLimit    uint64
	ResumeAt     uint64
	Paused       bool
}

// Status reads the configured cgroup metric and reports the same hysteresis
// state used by CanLease. It does not use host-wide memory as a substitute.
func (g *MemoryGovernor) Status(ctx context.Context) (MemoryStatus, error) {
	if g == nil || g.Reader == nil || g.SoftLimit == 0 || g.ResumeAt >= g.SoftLimit {
		return MemoryStatus{}, errors.New("memory governor is not configured")
	}
	current, err := g.Reader.CurrentBytes(ctx)
	if err != nil {
		return MemoryStatus{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.observe(current)
	return MemoryStatus{CurrentBytes: current, SoftLimit: g.SoftLimit, ResumeAt: g.ResumeAt, Paused: g.paused}, nil
}

func (g *MemoryGovernor) CanLease(ctx context.Context) (bool, string, error) {
	status, err := g.Status(ctx)
	if err != nil {
		if g == nil || g.Reader == nil || g.SoftLimit == 0 || g.ResumeAt >= g.SoftLimit {
			return false, "memory_governor_unconfigured", err
		}
		return false, "memory_metric_unavailable", err
	}
	if status.Paused {
		return false, "memory_pressure", nil
	}
	return true, "", nil
}

func (g *MemoryGovernor) observe(current uint64) {
	if current >= g.SoftLimit {
		g.paused = true
	}
	if g.paused && current < g.ResumeAt {
		g.paused = false
	}
}
