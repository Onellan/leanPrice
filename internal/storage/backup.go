package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
)

// BackupManifest identifies a verified SQLite snapshot by its full digest and
// the schema version embedded in that snapshot.
type BackupManifest struct {
	FormatVersion int       `json:"format_version"`
	SchemaVersion int       `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	SizeBytes     int64     `json:"size_bytes"`
	SHA256        string    `json:"sha256"`
}

// Backup makes a WAL-consistent online snapshot. The database is published
// first and the manifest is renamed last; a manifest marks a complete artifact.
func (s *Store) Backup(ctx context.Context, destination string) (BackupManifest, error) {
	if s == nil || s.db == nil {
		return BackupManifest{}, errors.New("storage: database is not open")
	}
	return BackupDatabase(ctx, s.path, destination)
}

// BackupDatabase makes a verified snapshot without requiring the application
// process lock. It is intended for the local operator command and online
// backup route; SQLite coordinates the snapshot with active writers.
func BackupDatabase(ctx context.Context, source, destination string) (BackupManifest, error) {
	if ctx == nil {
		return BackupManifest{}, errors.New("storage: context is required")
	}
	absSource, err := filepath.Abs(source)
	if err != nil {
		return BackupManifest{}, fmt.Errorf("storage: resolve database path: %w", err)
	}
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		return BackupManifest{}, fmt.Errorf("storage: resolve backup destination: %w", err)
	}
	if absDestination == absSource {
		return BackupManifest{}, errors.New("storage: backup destination must differ from the live database")
	}
	info, err := os.Lstat(absSource)
	if err != nil {
		return BackupManifest{}, fmt.Errorf("storage: inspect database file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return BackupManifest{}, errors.New("storage: database path must be a regular file")
	}
	manifestPath := absDestination + ".manifest.json"
	for _, path := range []string{absDestination, manifestPath} {
		if _, err := os.Lstat(path); err == nil {
			return BackupManifest{}, fmt.Errorf("storage: backup artifact already exists: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return BackupManifest{}, fmt.Errorf("storage: inspect backup path: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(absDestination), 0o700); err != nil {
		return BackupManifest{}, fmt.Errorf("storage: create backup directory: %w", err)
	}
	backupFile, err := os.CreateTemp(filepath.Dir(absDestination), ".leanprice-backup-*.sqlite")
	if err != nil {
		return BackupManifest{}, fmt.Errorf("storage: create temporary backup: %w", err)
	}
	tempPath := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return BackupManifest{}, fmt.Errorf("storage: close temporary backup: %w", err)
	}
	if err := os.Remove(tempPath); err != nil {
		return BackupManifest{}, fmt.Errorf("storage: prepare temporary backup path: %w", err)
	}
	manifestTemp, err := os.CreateTemp(filepath.Dir(absDestination), ".leanprice-manifest-*.json")
	if err != nil {
		return BackupManifest{}, fmt.Errorf("storage: create temporary manifest: %w", err)
	}
	manifestTempPath := manifestTemp.Name()
	if err := manifestTemp.Close(); err != nil {
		_ = os.Remove(tempPath)
		_ = os.Remove(manifestTempPath)
		return BackupManifest{}, fmt.Errorf("storage: close temporary manifest: %w", err)
	}
	_ = os.Remove(manifestTempPath)
	cleanup := func() {
		_ = os.Remove(tempPath)
		_ = os.Remove(tempPath + "-wal")
		_ = os.Remove(tempPath + "-shm")
		_ = os.Remove(manifestTempPath)
		_ = os.Remove(absDestination)
		_ = os.Remove(manifestPath)
	}
	if err := createOnlineSnapshot(ctx, absSource, tempPath); err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: create SQLite snapshot: %w", err)
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: restrict backup permissions: %w", err)
	}
	version, err := verifyDatabaseFile(ctx, tempPath)
	if err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: verify backup snapshot: %w", err)
	}
	hash, size, err := hashFile(tempPath)
	if err != nil {
		cleanup()
		return BackupManifest{}, err
	}
	manifest := BackupManifest{
		FormatVersion: 1,
		SchemaVersion: version,
		CreatedAt:     time.Now().UTC(),
		SizeBytes:     size,
		SHA256:        hash,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: encode backup manifest: %w", err)
	}
	manifestFile, err := os.OpenFile(manifestTempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: write backup manifest: %w", err)
	}
	if _, err := manifestFile.Write(append(manifestBytes, '\n')); err != nil {
		_ = manifestFile.Close()
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: write backup manifest: %w", err)
	}
	if err := manifestFile.Sync(); err != nil {
		_ = manifestFile.Close()
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: sync backup manifest: %w", err)
	}
	if err := manifestFile.Close(); err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: close backup manifest: %w", err)
	}
	if err := os.Rename(tempPath, absDestination); err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: publish backup: %w", err)
	}
	if err := os.Rename(manifestTempPath, manifestPath); err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: publish backup manifest: %w", err)
	}
	if err := syncDirectory(filepath.Dir(absDestination)); err != nil {
		cleanup()
		return BackupManifest{}, fmt.Errorf("storage: sync backup directory: %w", err)
	}
	return manifest, nil
}

func createOnlineSnapshot(ctx context.Context, sourcePath, destinationPath string) error {
	backupDB, err := sql.Open("sqlite", sqliteDSN(sourcePath))
	if err != nil {
		return fmt.Errorf("open backup source: %w", err)
	}
	backupDB.SetMaxOpenConns(1)
	backupDB.SetMaxIdleConns(1)
	defer backupDB.Close()
	connection, err := backupDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve backup source connection: %w", err)
	}
	defer connection.Close()
	destinationURI := sqliteDSN(destinationPath)
	return connection.Raw(func(raw any) error {
		backuper, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("sqlite driver does not support online backup")
		}
		backup, err := backuper.NewBackup(destinationURI)
		if err != nil {
			return fmt.Errorf("start online backup: %w", err)
		}
		finished := false
		finish := func() error {
			if finished {
				return nil
			}
			finished = true
			return backup.Finish()
		}
		defer func() { _ = finish() }()
		for retries := 0; ; {
			if err := ctx.Err(); err != nil {
				return errors.Join(err, finish())
			}
			more, stepErr := backup.Step(128)
			if stepErr != nil {
				if retries >= 100 || !retryableBackupError(stepErr) {
					return errors.Join(stepErr, finish())
				}
				retries++
				timer := time.NewTimer(10 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					return errors.Join(ctx.Err(), finish())
				case <-timer.C:
				}
				continue
			}
			retries = 0
			if !more {
				return finish()
			}
		}
	})
}

func retryableBackupError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy")
}

// ValidateBackup verifies the adjacent manifest, digest, schema version and
// SQLite integrity without trusting filenames in the manifest.
func ValidateBackup(ctx context.Context, path string) (BackupManifest, error) {
	var manifest BackupManifest
	absPath, err := filepath.Abs(path)
	if err != nil {
		return manifest, fmt.Errorf("storage: resolve backup: %w", err)
	}
	manifestBytes, err := os.ReadFile(absPath + ".manifest.json")
	if err != nil {
		return manifest, fmt.Errorf("storage: read backup manifest: %w", err)
	}
	if len(manifestBytes) > 4096 {
		return manifest, errors.New("storage: backup manifest exceeds size limit")
	}
	decoder := json.NewDecoder(strings.NewReader(string(manifestBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("storage: parse backup manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return manifest, errors.New("storage: backup manifest contains trailing data")
	}
	if manifest.FormatVersion != 1 || manifest.SchemaVersion < 1 || len(manifest.SHA256) != 64 || manifest.SizeBytes < 1 {
		return manifest, errors.New("storage: backup manifest fields are invalid")
	}
	hash, size, err := hashFile(absPath)
	if err != nil {
		return manifest, err
	}
	if hash != manifest.SHA256 || size != manifest.SizeBytes {
		return manifest, errors.New("storage: backup digest or size mismatch")
	}
	version, err := verifyDatabaseFile(ctx, absPath)
	if err != nil {
		return manifest, fmt.Errorf("storage: verify backup database: %w", err)
	}
	if version != manifest.SchemaVersion {
		return manifest, errors.New("storage: backup schema version does not match manifest")
	}
	return manifest, nil
}

func verifyDatabaseFile(ctx context.Context, path string) (int, error) {
	if ctx == nil {
		return 0, errors.New("storage: context is required")
	}
	if _, err := os.Stat(path); err != nil {
		return 0, fmt.Errorf("storage: stat database snapshot: %w", err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := url.Values{}
	query.Set("mode", "ro")
	u.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return 0, fmt.Errorf("storage: open database snapshot: %w", err)
	}
	defer db.Close()
	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check); err != nil {
		return 0, fmt.Errorf("storage: integrity check: %w", err)
	}
	if check != "ok" {
		return 0, fmt.Errorf("storage: integrity check reported %q", check)
	}
	foreignKeys, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return 0, fmt.Errorf("storage: foreign key check: %w", err)
	}
	if foreignKeys.Next() {
		var table string
		var rowID, parent, foreignKey any
		if err := foreignKeys.Scan(&table, &rowID, &parent, &foreignKey); err != nil {
			_ = foreignKeys.Close()
			return 0, fmt.Errorf("storage: read foreign key check: %w", err)
		}
		_ = foreignKeys.Close()
		return 0, fmt.Errorf("storage: foreign key check found a violation in %q", table)
	}
	if err := foreignKeys.Err(); err != nil {
		_ = foreignKeys.Close()
		return 0, fmt.Errorf("storage: read foreign key check: %w", err)
	}
	if err := foreignKeys.Close(); err != nil {
		return 0, fmt.Errorf("storage: close foreign key check: %w", err)
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("storage: read snapshot schema version: %w", err)
	}
	return version, nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("storage: open file for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("storage: checksum file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
