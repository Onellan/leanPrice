package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Onellan/leanPrice/internal/app"
	"github.com/Onellan/leanPrice/internal/auth"
	"github.com/Onellan/leanPrice/internal/storage"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := checkReady(); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "admin-password-reset" {
		if err := resetAdminPassword(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "backup" {
		if err := createBackup(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "restore" {
		if err := restoreBackup(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := app.LoadConfig(os.LookupEnv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("server stopped with an error", "error", err)
		os.Exit(1)
	}
}

func createBackup(destination string) error {
	cfg, err := app.LoadConfig(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	manifest, err := storage.BackupDatabase(context.Background(), filepath.Join(cfg.DataDir, "leanprice.sqlite"), destination)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Backup complete: %s\nManifest: %s.manifest.json\nSHA-256: %s\n", destination, destination, manifest.SHA256)
	return nil
}

func restoreBackup(source string) error {
	cfg, err := app.LoadConfig(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	result, err := storage.RestoreDatabase(context.Background(), filepath.Join(cfg.DataDir, "leanprice.sqlite"), source)
	if err != nil {
		return err
	}
	if result.SafetyBackupPath == "" {
		fmt.Fprintln(os.Stdout, "Restore complete. The database was newly created; no prior database needed a safety snapshot.")
		return nil
	}
	fmt.Fprintf(os.Stdout, "Restore complete. Safety backup: %s\nManifest: %s.manifest.json\n", result.SafetyBackupPath, result.SafetyBackupPath)
	return nil
}

func resetAdminPassword(username string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("usage: leanprice admin-password-reset <username>")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return fmt.Errorf("password reset requires an interactive terminal; stop the application first")
	}
	fmt.Fprint(os.Stderr, "New administrator password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	defer clear(password)
	fmt.Fprint(os.Stderr, "Confirm new administrator password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("read password confirmation: %w", err)
	}
	defer clear(confirmation)
	if len(password) != len(confirmation) || subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return fmt.Errorf("passwords do not match")
	}
	if err := auth.ValidatePassword(string(password)); err != nil {
		return err
	}
	cfg, err := app.LoadConfig(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("read configuration: %w", err)
	}
	store, err := storage.Open(context.Background(), filepath.Join(cfg.DataDir, "leanprice.sqlite"))
	if err != nil {
		return fmt.Errorf("open database (the application must be stopped): %w", err)
	}
	defer store.Close()
	admin, err := store.AdminByUsername(context.Background(), username)
	if err != nil {
		return fmt.Errorf("find administrator: %w", err)
	}
	hasher, err := auth.NewPasswordHasher()
	if err != nil {
		return fmt.Errorf("initialize password hashing: %w", err)
	}
	passwordHash, err := hasher.Hash(context.Background(), string(password))
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if err := store.ChangeAdminPassword(context.Background(), admin.ID, passwordHash, time.Now().UTC()); err != nil {
		return fmt.Errorf("update administrator password: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Password changed. All sessions were revoked.")
	return nil
}

func checkReady() error {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/health/ready")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("readiness endpoint returned %s", response.Status)
	}
	return nil
}
