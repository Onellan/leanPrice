package app

import (
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config contains validated process-level configuration. Webhook credentials
// are optional and are used only by the separate notification target client.
type Config struct {
	Address             string
	DataDir             string
	TrustedProxies      []net.IPNet
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	RequestTimeout      time.Duration
	ShutdownTimeout     time.Duration
	WebhookEndpoint     string
	WebhookToken        string
	WebhookPrivateCIDRs []netip.Prefix
	CgroupMemoryCurrent string
}

// LoadConfig reads environment values through getenv so configuration can be
// tested without mutating the process environment.
func LoadConfig(getenv func(string) (string, bool)) (Config, error) {
	if getenv == nil {
		return Config{}, fmt.Errorf("environment lookup is required")
	}
	cfg := Config{
		Address:             "127.0.0.1:8080",
		DataDir:             "/data",
		ReadHeaderTimeout:   5 * time.Second,
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        30 * time.Second,
		IdleTimeout:         60 * time.Second,
		RequestTimeout:      20 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		CgroupMemoryCurrent: "/sys/fs/cgroup/memory.current",
	}
	if value, ok := getenv("LEANPRICE_ADDR"); ok {
		cfg.Address = strings.TrimSpace(value)
	}
	if value, ok := getenv("LEANPRICE_DATA_DIR"); ok {
		cfg.DataDir = strings.TrimSpace(value)
	}
	if value, ok := getenv("LEANPRICE_TRUSTED_PROXIES"); ok {
		proxies, err := parseTrustedProxies(value)
		if err != nil {
			return Config{}, err
		}
		cfg.TrustedProxies = proxies
	}
	if value, ok := getenv("LEANPRICE_WEBHOOK_URL"); ok {
		cfg.WebhookEndpoint = strings.TrimSpace(value)
		if len(cfg.WebhookEndpoint) > 2048 {
			return Config{}, fmt.Errorf("LEANPRICE_WEBHOOK_URL exceeds 2048 characters")
		}
	}
	if value, ok := getenv("LEANPRICE_WEBHOOK_TOKEN"); ok {
		cfg.WebhookToken = strings.TrimSpace(value)
		if len(cfg.WebhookToken) > 2048 {
			return Config{}, fmt.Errorf("LEANPRICE_WEBHOOK_TOKEN exceeds 2048 characters")
		}
	}
	if value, ok := getenv("LEANPRICE_WEBHOOK_PRIVATE_CIDRS"); ok {
		cidrs, err := parseWebhookPrivateCIDRs(value)
		if err != nil {
			return Config{}, err
		}
		cfg.WebhookPrivateCIDRs = cidrs
	}
	if value, ok := getenv("LEANPRICE_CGROUP_MEMORY_CURRENT"); ok {
		cfg.CgroupMemoryCurrent = strings.TrimSpace(value)
		if cfg.CgroupMemoryCurrent == "" {
			return Config{}, fmt.Errorf("LEANPRICE_CGROUP_MEMORY_CURRENT must name a cgroup v2 memory.current file")
		}
	}
	if value, ok := getenv("LEANPRICE_READ_HEADER_TIMEOUT"); ok {
		parsed, err := parseDuration("LEANPRICE_READ_HEADER_TIMEOUT", value)
		if err != nil {
			return Config{}, err
		}
		cfg.ReadHeaderTimeout = parsed
	}
	if value, ok := getenv("LEANPRICE_READ_TIMEOUT"); ok {
		parsed, err := parseDuration("LEANPRICE_READ_TIMEOUT", value)
		if err != nil {
			return Config{}, err
		}
		cfg.ReadTimeout = parsed
	}
	if value, ok := getenv("LEANPRICE_WRITE_TIMEOUT"); ok {
		parsed, err := parseDuration("LEANPRICE_WRITE_TIMEOUT", value)
		if err != nil {
			return Config{}, err
		}
		cfg.WriteTimeout = parsed
	}
	if value, ok := getenv("LEANPRICE_IDLE_TIMEOUT"); ok {
		parsed, err := parseDuration("LEANPRICE_IDLE_TIMEOUT", value)
		if err != nil {
			return Config{}, err
		}
		cfg.IdleTimeout = parsed
	}
	if value, ok := getenv("LEANPRICE_REQUEST_TIMEOUT"); ok {
		parsed, err := parseDuration("LEANPRICE_REQUEST_TIMEOUT", value)
		if err != nil {
			return Config{}, err
		}
		cfg.RequestTimeout = parsed
	}
	if value, ok := getenv("LEANPRICE_SHUTDOWN_TIMEOUT"); ok {
		parsed, err := parseDuration("LEANPRICE_SHUTDOWN_TIMEOUT", value)
		if err != nil {
			return Config{}, err
		}
		cfg.ShutdownTimeout = parsed
	}
	if _, port, err := net.SplitHostPort(cfg.Address); err != nil {
		return Config{}, fmt.Errorf("LEANPRICE_ADDR must be host:port: %w", err)
	} else {
		portNumber, parseErr := strconv.Atoi(port)
		if parseErr != nil || portNumber < 1 || portNumber > 65535 {
			return Config{}, fmt.Errorf("LEANPRICE_ADDR port must be between 1 and 65535")
		}
	}
	if cfg.DataDir == "" || cfg.DataDir == "." {
		return Config{}, fmt.Errorf("LEANPRICE_DATA_DIR must name a directory")
	}
	if cfg.WebhookEndpoint == "" && cfg.WebhookToken != "" {
		return Config{}, fmt.Errorf("LEANPRICE_WEBHOOK_TOKEN requires LEANPRICE_WEBHOOK_URL")
	}
	cfg.DataDir = filepath.Clean(cfg.DataDir)
	if cfg.ReadHeaderTimeout <= 0 || cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 || cfg.IdleTimeout <= 0 || cfg.RequestTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP and shutdown timeouts must be positive")
	}
	return cfg, nil
}

func parseWebhookPrivateCIDRs(value string) ([]netip.Prefix, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	entries := strings.Split(value, ",")
	if len(entries) > 16 {
		return nil, fmt.Errorf("LEANPRICE_WEBHOOK_PRIVATE_CIDRS supports at most 16 networks")
	}
	result := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(entry))
		if err != nil || !prefix.Masked().Addr().IsPrivate() && !prefix.Masked().Addr().IsLoopback() {
			return nil, fmt.Errorf("LEANPRICE_WEBHOOK_PRIVATE_CIDRS must contain private or loopback CIDRs")
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}

func parseTrustedProxies(value string) ([]net.IPNet, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	entries := strings.Split(value, ",")
	if len(entries) > 16 {
		return nil, fmt.Errorf("LEANPRICE_TRUSTED_PROXIES supports at most 16 networks")
	}
	proxies := make([]net.IPNet, 0, len(entries))
	for _, entry := range entries {
		_, network, err := net.ParseCIDR(strings.TrimSpace(entry))
		if err != nil {
			return nil, fmt.Errorf("LEANPRICE_TRUSTED_PROXIES contains an invalid CIDR")
		}
		proxies = append(proxies, *network)
	}
	return proxies, nil
}

func parseDuration(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 || duration > 5*time.Minute {
		return 0, fmt.Errorf("%s must be a duration between 0 and 5m", name)
	}
	return duration, nil
}
