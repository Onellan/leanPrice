// Package storage owns the durable SQLite boundary. Network work must happen
// before entering a storage transaction.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	_ "modernc.org/sqlite"
)

// Store is the single-connection database handle used by the application.
// SQLite serializes writers, and one connection makes connection-local safety
// pragmas predictable while avoiding a pool of competing writers on a Pi.
type Store struct {
	db   *sql.DB
	path string
	lock *os.File
}

// Open creates or opens a file-backed database and applies all forward
// migrations. The caller owns the returned handle and must close it.
func Open(ctx context.Context, path string) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("storage: context is required")
	}
	path = strings.TrimSpace(path)
	if path == "" || path == ":memory:" {
		return nil, errors.New("storage: a file-backed database path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return nil, fmt.Errorf("storage: create database directory: %w", err)
	}
	if info, err := os.Lstat(absPath); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("storage: database path must be a regular file, not a symlink or special file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("storage: inspect database path: %w", err)
	}
	file, err := os.OpenFile(absPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("storage: create database file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("storage: restrict database file permissions: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("storage: close database file: %w", err)
	}
	lock, err := acquireProcessLock(absPath + ".lock")
	if err != nil {
		return nil, err
	}

	dsn := sqliteDSN(absPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = releaseProcessLock(lock)
		return nil, fmt.Errorf("storage: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	store := &Store{db: db, path: absPath, lock: lock}
	cleanup := func(openErr error) (*Store, error) {
		_ = db.Close()
		_ = releaseProcessLock(store.lock)
		store.lock = nil
		return nil, openErr
	}
	if err := db.PingContext(ctx); err != nil {
		return cleanup(fmt.Errorf("storage: ping sqlite: %w", err))
	}
	if err := store.Migrate(ctx); err != nil {
		return cleanup(err)
	}
	if err := os.Chmod(absPath, 0o600); err != nil {
		return cleanup(fmt.Errorf("storage: restrict database file permissions: %w", err))
	}
	if err := store.restrictSidecarPermissions(); err != nil {
		return cleanup(err)
	}
	return store, nil
}

func sqliteDSN(path string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := url.Values{}
	query.Set("_txlock", "immediate")
	for _, pragma := range []string{
		"foreign_keys(1)",
		"busy_timeout(5000)",
		"journal_mode(WAL)",
		"synchronous(NORMAL)",
		"cache_size(-4096)",
		"mmap_size(0)",
		"wal_autocheckpoint(1000)",
		"trusted_schema(OFF)",
	} {
		query.Add("_pragma", pragma)
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func (s *Store) restrictSidecarPermissions() error {
	for _, suffix := range []string{"-wal", "-shm"} {
		path := s.path + suffix
		if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("storage: restrict %s permissions: %w", suffix[1:], err)
		}
	}
	return nil
}

// Close closes the underlying SQLite pool and checkpoint resources owned by
// SQLite. Callers should stop workers before closing the store.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	dbErr := s.db.Close()
	lockErr := releaseProcessLock(s.lock)
	s.lock = nil
	if dbErr != nil {
		return dbErr
	}
	return lockErr
}

func acquireProcessLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("storage: open database process lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("storage: restrict database process lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("storage: database is already open by another leanPrice process: %w", err)
	}
	return file, nil
}

func releaseProcessLock(file *os.File) error {
	if file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	closeErr := file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

// Path returns the absolute database filename, primarily for backup tooling.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}
