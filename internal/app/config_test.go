package app

import (
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Address != "127.0.0.1:8080" {
		t.Fatalf("Address = %q, want 127.0.0.1:8080", cfg.Address)
	}
	if cfg.DataDir != "/data" {
		t.Fatalf("DataDir = %q, want /data", cfg.DataDir)
	}
	if cfg.ReadHeaderTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		t.Fatalf("timeouts must be positive: %+v", cfg)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]string
	}{
		{name: "invalid port", values: map[string]string{"LEANPRICE_ADDR": "127.0.0.1:0"}},
		{name: "empty data directory", values: map[string]string{"LEANPRICE_DATA_DIR": "  "}},
		{name: "invalid timeout", values: map[string]string{"LEANPRICE_SHUTDOWN_TIMEOUT": "not-a-duration"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(func(key string) (string, bool) {
				value, ok := tt.values[key]
				return value, ok
			})
			if err == nil {
				t.Fatal("LoadConfig() error = nil, want validation error")
			}
		})
	}
}

func TestLoadConfigRestrictsTrustedProxyCIDRs(t *testing.T) {
	valid, err := LoadConfig(func(key string) (string, bool) {
		if key == "LEANPRICE_TRUSTED_PROXIES" {
			return "192.0.2.10/32, 2001:db8::/64", true
		}
		return "", false
	})
	if err != nil || len(valid.TrustedProxies) != 2 {
		t.Fatalf("valid proxy CIDRs = %v, %v", valid.TrustedProxies, err)
	}
	if _, err := LoadConfig(func(key string) (string, bool) {
		if key == "LEANPRICE_TRUSTED_PROXIES" {
			return "not-a-network", true
		}
		return "", false
	}); err == nil {
		t.Fatal("invalid trusted proxy CIDR was accepted")
	}
}
