package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestRunServesAndShutsDownOnCancellation(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve address: %v", err)
	}
	address := probe.Addr().String()
	dataDir := filepath.Join(t.TempDir(), "data")
	if err := probe.Close(); err != nil {
		t.Fatalf("release address: %v", err)
	}
	cfg, err := LoadConfig(func(key string) (string, bool) {
		switch key {
		case "LEANPRICE_ADDR":
			return address, true
		case "LEANPRICE_DATA_DIR":
			return dataDir, true
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go func() { finished <- Run(ctx, cfg, logger) }()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, requestErr := client.Get("http://" + address + "/health/ready")
		if requestErr == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("readiness status = %d, want 200", response.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready: %v", requestErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("Run() = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}
