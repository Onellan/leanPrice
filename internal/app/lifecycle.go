package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Onellan/leanPrice/internal/alerts"
	"github.com/Onellan/leanPrice/internal/auth"
	"github.com/Onellan/leanPrice/internal/catalog"
	"github.com/Onellan/leanPrice/internal/fetch"
	"github.com/Onellan/leanPrice/internal/httpui"
	"github.com/Onellan/leanPrice/internal/jobs"
	"github.com/Onellan/leanPrice/internal/notify"
	"github.com/Onellan/leanPrice/internal/ops"
	"github.com/Onellan/leanPrice/internal/storage"
)

// Run serves the local SSR application until the process context is canceled.
func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	database, err := storage.Open(ctx, filepath.Join(cfg.DataDir, "leanprice.sqlite"))
	if err != nil {
		return fmt.Errorf("open local database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("close local database", "error", err)
		}
	}()
	passwordHasher, err := auth.NewPasswordHasher()
	if err != nil {
		return fmt.Errorf("initialize password hashing: %w", err)
	}
	authService, err := auth.NewService(database, passwordHasher)
	if err != nil {
		return fmt.Errorf("initialize authentication: %w", err)
	}
	setupTokenPath := filepath.Join(cfg.DataDir, "setup-token")
	if err := prepareSetupToken(ctx, database, authService, setupTokenPath, logger); err != nil {
		return fmt.Errorf("prepare first-run setup: %w", err)
	}
	if _, err := database.DeleteExpiredSessions(ctx, time.Now().UTC()); err != nil {
		return fmt.Errorf("clean expired sessions: %w", err)
	}
	fetchClient, err := fetch.NewClient(fetch.Config{Policy: database, RateGate: database})
	if err != nil {
		return fmt.Errorf("initialize approved-source fetch client: %w", err)
	}
	refresher := catalog.Refresher{Repository: database, Fetcher: fetchClient}
	destinationKey := "in_app"
	var deliveryService *notify.DeliveryService
	var testNotification func(context.Context, string) error
	if cfg.WebhookEndpoint != "" {
		webhook, webhookErr := notify.NewClient(notify.Config{Endpoint: cfg.WebhookEndpoint, BearerToken: cfg.WebhookToken, AllowedPrivateCIDRs: cfg.WebhookPrivateCIDRs})
		if webhookErr != nil {
			return fmt.Errorf("initialize notification webhook target: %w", webhookErr)
		}
		testNotification = func(testCtx context.Context, key string) error {
			response, sendErr := webhook.SendTest(testCtx, key)
			if sendErr != nil {
				return sendErr
			}
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				return errors.New("notify: webhook test returned a non-success status")
			}
			return nil
		}
		destinationKey = "webhook"
		deliveryService = &notify.DeliveryService{
			Repository: database, Sender: webhook, Gate: &ops.MemoryGovernor{Reader: ops.CgroupV2Reader{Path: cfg.CgroupMemoryCurrent}, SoftLimit: 350_000_000, ResumeAt: 320_000_000}, WorkerID: "notify-1",
			OnDeferred: func(_ context.Context, reason string, _ time.Time) error {
				logger.Warn("notification delivery deferred", "reason", reason)
				return nil
			},
		}
	}
	alertEngine := alerts.Engine{Repository: database, DestinationKey: destinationKey}
	resourceGate := &ops.MemoryGovernor{Reader: ops.CgroupV2Reader{Path: cfg.CgroupMemoryCurrent}, SoftLimit: 350_000_000, ResumeAt: 320_000_000}
	scheduler := jobs.Scheduler{
		Repository: database, Gate: resourceGate, Processor: applicationProcessor{refresher: refresher, alerts: alertEngine},
		WorkerID: "worker-1", Lease: 2 * time.Minute, PollInterval: time.Second,
		BeforeRun: func(scheduleCtx context.Context, now time.Time) error {
			_, err := database.ScheduleRefreshes(scheduleCtx, now, 24*time.Hour, 100)
			return err
		},
		OnDeferred: func(_ context.Context, reason string, _ time.Time) error {
			logger.Warn("background work deferred", "reason", reason)
			return nil
		},
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Address, err)
	}
	var ready atomic.Bool
	router, err := httpui.NewRouter(httpui.Options{
		Ready: ready.Load, RequestTimeout: cfg.RequestTimeout, Auth: authService,
		Store: database, SetupTokenPath: setupTokenPath, TrustedProxies: cfg.TrustedProxies,
		TestNotification: testNotification,
		MemoryStatus: func(statusCtx context.Context) (httpui.ResourceStatus, error) {
			status, statusErr := resourceGate.Status(statusCtx)
			return httpui.ResourceStatus{CurrentBytes: status.CurrentBytes, SoftLimit: status.SoftLimit, ResumeAt: status.ResumeAt, Paused: status.Paused}, statusErr
		},
	})
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("initialize HTTP routes: %w", err)
	}
	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	ready.Store(true)
	logger.Info("leanPrice server started", "address", listener.Addr().String())
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	workerDone := make(chan struct{})
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	go func() {
		defer close(workerDone)
		workerTicker := time.NewTicker(time.Second)
		defer workerTicker.Stop()
		for {
			if deliveryService != nil {
				if _, err := deliveryService.RunOnce(workerCtx); err != nil && workerCtx.Err() == nil {
					logger.Warn("notification delivery worker failed", "error", err)
				}
			}
			if _, err := scheduler.RunOnce(workerCtx); err != nil && workerCtx.Err() == nil {
				logger.Warn("background job worker failed", "error", err)
			}
			select {
			case <-workerCtx.Done():
				return
			case <-workerTicker.C:
			}
		}
	}()

	select {
	case err := <-serveResult:
		stopWorker()
		<-workerDone
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	ready.Store(false)
	stopWorker()
	<-workerDone
	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Warn("graceful shutdown timed out; closing active connections", "error", err)
		_ = server.Close()
	}
	if err := <-serveResult; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("stop HTTP server: %w", err)
	}
	logger.Info("leanPrice server stopped")
	return nil
}

type applicationProcessor struct {
	refresher catalog.Refresher
	alerts    alerts.Engine
}

func (p applicationProcessor) Process(ctx context.Context, job storage.Job) error {
	switch job.Kind {
	case "refresh_listing":
		_, err := p.refresher.Refresh(ctx, job.ListingID)
		if err != nil {
			var fetchErr *fetch.FetchError
			if errors.As(err, &fetchErr) {
				switch fetchErr.Kind {
				case fetch.KindPolicy, fetch.KindBlocked:
					return &jobs.Failure{Status: "blocked", Code: "source_blocked", Cause: err}
				case fetch.KindRateLimited:
					delay := fetchErr.RetryAfter
					if delay <= 0 {
						delay = 30 * time.Second
					}
					return &jobs.Failure{Status: "retry_at", Code: "source_rate_limited", RetryAfter: delay, Cause: err}
				case fetch.KindOversized, fetch.KindInvalid:
					return &jobs.Failure{Status: "blocked", Code: "source_response_rejected", Cause: err}
				}
			}
			return err
		}
		_, err = p.alerts.Evaluate(ctx)
		return err
	case "maintenance":
		_, err := p.alerts.Evaluate(ctx)
		return err
	default:
		return &jobs.Failure{Status: "dead", Code: "unsupported_job_kind", Cause: fmt.Errorf("unsupported background job kind")}
	}
}

func prepareSetupToken(ctx context.Context, store *storage.Store, service *auth.Service, path string, logger *slog.Logger) error {
	hasAdmin, err := store.HasAdmin(ctx)
	if err != nil {
		return err
	}
	if hasAdmin {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove consumed setup token file: %w", err)
		}
		return nil
	}
	if token, err := readSetupToken(path); err == nil {
		matches, matchErr := service.SetupTokenMatches(ctx, token)
		if matchErr != nil {
			return matchErr
		}
		if matches {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	token, err := auth.NewSetupToken()
	if err != nil {
		return fmt.Errorf("generate setup token: %w", err)
	}
	if err := writeSetupToken(path, token); err != nil {
		return err
	}
	if err := service.SetSetupToken(ctx, token, time.Now().UTC()); err != nil {
		_ = os.Remove(path)
		return err
	}
	logger.Info("first administrator setup is available", "token_file", path)
	return nil
}

func readSetupToken(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 128 {
		return "", fmt.Errorf("setup token file permissions or size are unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	value, err := io.ReadAll(io.LimitReader(file, 129))
	if err != nil {
		return "", fmt.Errorf("read setup token file: %w", err)
	}
	if len(value) > 128 {
		return "", fmt.Errorf("setup token file exceeds size limit")
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(value), "\n"), "\r"), nil
}

func writeSetupToken(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create setup token directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".setup-token-*")
	if err != nil {
		return fmt.Errorf("create setup token file: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("restrict setup token file: %w", err)
	}
	if _, err := temporary.WriteString(token + "\n"); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("write setup token file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("sync setup token file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("close setup token file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("publish setup token file: %w", err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open setup token directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync setup token directory: %w", err)
	}
	return nil
}
