package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maximumRestoreBytes int64 = 2 * 1024 * 1024 * 1024

// RestoreResult identifies the retained pre-restore safety snapshot. An empty
// path means the target did not exist before restoration.
type RestoreResult struct {
	SafetyBackupPath string
	SafetyBackup     BackupManifest
}

// RestoreDatabase replaces a stopped instance's database from a verified
// snapshot. Every leanPrice process must use Store.Open so the advisory lock
// can enforce the stop-the-world requirement. A verified safety snapshot of
// the current database is retained and used if replacement fails.
func RestoreDatabase(ctx context.Context, databasePath, backupPath string) (RestoreResult, error) {
	if ctx == nil {
		return RestoreResult{}, errors.New("storage: context is required")
	}
	target, err := filepath.Abs(databasePath)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("storage: resolve restore target: %w", err)
	}
	source, err := filepath.Abs(backupPath)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("storage: resolve restore source: %w", err)
	}
	if target == source {
		return RestoreResult{}, errors.New("storage: restore source and target must differ")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return RestoreResult{}, fmt.Errorf("storage: create restore directory: %w", err)
	}
	lock, err := acquireProcessLock(target + ".lock")
	if err != nil {
		return RestoreResult{}, err
	}
	defer func() { _ = releaseProcessLock(lock) }()
	manifest, err := ValidateBackup(ctx, source)
	if err != nil {
		return RestoreResult{}, err
	}
	if manifest.SizeBytes > maximumRestoreBytes {
		return RestoreResult{}, fmt.Errorf("storage: backup exceeds restore limit of %d bytes", maximumRestoreBytes)
	}
	if realSource, err := filepath.EvalSymlinks(source); err == nil {
		if realTarget, targetErr := filepath.EvalSymlinks(target); targetErr == nil && realSource == realTarget {
			return RestoreResult{}, errors.New("storage: restore source resolves to the database target")
		}
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return RestoreResult{}, errors.New("storage: restore target is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestoreResult{}, fmt.Errorf("storage: inspect restore target: %w", err)
	}

	targetExists := false
	if _, err := os.Stat(target); err == nil {
		targetExists = true
	}
	safetyPath := ""
	var safetyManifest BackupManifest
	if targetExists {
		safetyPath = target + ".pre-restore-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".sqlite"
		safetyManifest, err = snapshotDatabase(ctx, target, safetyPath)
		if err != nil {
			return RestoreResult{}, fmt.Errorf("storage: preserve current database: %w", err)
		}
	}

	stage, err := makeSiblingTemp(target, ".restore-*.sqlite")
	if err != nil {
		return RestoreResult{}, err
	}
	defer removeSQLiteFiles(stage)
	if err := copyFileLimited(ctx, source, stage, manifest.SizeBytes); err != nil {
		return RestoreResult{}, fmt.Errorf("storage: stage backup: %w", err)
	}
	if err := os.Chmod(stage, 0o600); err != nil {
		return RestoreResult{}, fmt.Errorf("storage: restrict staged backup: %w", err)
	}
	if err := verifyStagedBackup(ctx, stage, manifest); err != nil {
		return RestoreResult{}, err
	}
	// Open applies any supported forward migrations to the staged copy before
	// the current database is touched.
	stagedStore, err := Open(ctx, stage)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("storage: validate and upgrade staged backup: %w", err)
	}
	if err := stagedStore.Close(); err != nil {
		return RestoreResult{}, fmt.Errorf("storage: close staged database: %w", err)
	}
	if err := removeSQLiteSidecars(stage); err != nil {
		return RestoreResult{}, fmt.Errorf("storage: clean staged database sidecars: %w", err)
	}
	if err := os.Remove(stage + ".lock"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return RestoreResult{}, fmt.Errorf("storage: clean staged process lock: %w", err)
	}
	if _, err := verifyDatabaseFile(ctx, stage); err != nil {
		return RestoreResult{}, fmt.Errorf("storage: verify migrated staged database: %w", err)
	}
	if err := removeSQLiteSidecars(target); err != nil {
		if targetExists {
			return RestoreResult{}, recoverRestoreTarget(ctx, target, safetyPath, fmt.Errorf("storage: remove old database sidecars: %w", err))
		}
		return RestoreResult{}, fmt.Errorf("storage: remove old database sidecars: %w", err)
	}
	if err := os.Rename(stage, target); err != nil {
		if targetExists {
			return RestoreResult{}, recoverRestoreTarget(ctx, target, safetyPath, fmt.Errorf("storage: install restored database: %w", err))
		}
		return RestoreResult{}, fmt.Errorf("storage: install restored database: %w", err)
	}
	if err := os.Chmod(target, 0o600); err != nil {
		if targetExists {
			return RestoreResult{}, recoverRestoreTarget(ctx, target, safetyPath, fmt.Errorf("storage: restrict restored database: %w", err))
		}
		return RestoreResult{}, fmt.Errorf("storage: restrict restored database: %w", err)
	}
	if err := syncDirectory(filepath.Dir(target)); err != nil {
		if targetExists {
			return RestoreResult{}, recoverRestoreTarget(ctx, target, safetyPath, fmt.Errorf("storage: sync restored database directory: %w", err))
		}
		return RestoreResult{}, fmt.Errorf("storage: sync restored database directory: %w", err)
	}
	return RestoreResult{SafetyBackupPath: safetyPath, SafetyBackup: safetyManifest}, nil
}

func verifyStagedBackup(ctx context.Context, path string, manifest BackupManifest) error {
	hash, size, err := hashFile(path)
	if err != nil {
		return err
	}
	if hash != manifest.SHA256 || size != manifest.SizeBytes {
		return errors.New("storage: staged backup digest or size mismatch")
	}
	version, err := verifyDatabaseFile(ctx, path)
	if err != nil {
		return fmt.Errorf("storage: verify staged backup: %w", err)
	}
	if version != manifest.SchemaVersion {
		return errors.New("storage: staged backup schema version mismatch")
	}
	return nil
}

func snapshotDatabase(ctx context.Context, source, destination string) (BackupManifest, error) {
	if _, err := os.Lstat(destination); err == nil {
		return BackupManifest{}, fmt.Errorf("storage: safety snapshot already exists: %s", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return BackupManifest{}, err
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".leanprice-safety-*.sqlite")
	if err != nil {
		return BackupManifest{}, fmt.Errorf("storage: create safety snapshot: %w", err)
	}
	temp := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, err
	}
	if err := os.Remove(temp); err != nil {
		return BackupManifest{}, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(source))
	if err != nil {
		return BackupManifest{}, fmt.Errorf("storage: open current database for safety snapshot: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return BackupManifest{}, fmt.Errorf("storage: read current database for safety snapshot: %w", err)
	}
	_, snapshotErr := db.ExecContext(ctx, "VACUUM INTO ?", temp)
	closeErr := db.Close()
	if snapshotErr != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, fmt.Errorf("storage: snapshot current database: %w", snapshotErr)
	}
	if closeErr != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, fmt.Errorf("storage: close current database snapshot: %w", closeErr)
	}
	if err := os.Chmod(temp, 0o600); err != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, err
	}
	version, err := verifyDatabaseFile(ctx, temp)
	if err != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, err
	}
	hash, size, err := hashFile(temp)
	if err != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, err
	}
	manifest := BackupManifest{FormatVersion: 1, SchemaVersion: version, CreatedAt: time.Now().UTC(), SizeBytes: size, SHA256: hash}
	bytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, err
	}
	manifestPath := destination + ".manifest.json"
	manifestTemp := manifestPath + ".tmp"
	if err := os.WriteFile(manifestTemp, append(bytes, '\n'), 0o600); err != nil {
		_ = os.Remove(temp)
		return BackupManifest{}, fmt.Errorf("storage: write safety manifest: %w", err)
	}
	if err := os.Rename(temp, destination); err != nil {
		_ = os.Remove(manifestTemp)
		_ = os.Remove(temp)
		return BackupManifest{}, fmt.Errorf("storage: publish safety snapshot: %w", err)
	}
	if err := os.Rename(manifestTemp, manifestPath); err != nil {
		_ = os.Remove(manifestTemp)
		return BackupManifest{}, fmt.Errorf("storage: publish safety manifest: %w", err)
	}
	if err := syncDirectory(filepath.Dir(destination)); err != nil {
		return BackupManifest{}, err
	}
	return manifest, nil
}

func makeSiblingTemp(target, pattern string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(target), pattern)
	if err != nil {
		return "", fmt.Errorf("storage: create restore staging file: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func copyFileLimited(ctx context.Context, source, destination string, expectedSize int64) error {
	if expectedSize < 1 || expectedSize > maximumRestoreBytes {
		return errors.New("storage: invalid restore byte count")
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("storage: open backup: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("storage: create staged restore: %w", err)
	}
	limited := &contextReader{ctx: ctx, reader: io.LimitReader(in, expectedSize+1)}
	size, copyErr := io.Copy(out, limited)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("storage: copy backup: %w", copyErr)
	}
	if syncErr != nil {
		return fmt.Errorf("storage: sync staged restore: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("storage: close staged restore: %w", closeErr)
	}
	if size != expectedSize {
		return fmt.Errorf("storage: copied %d bytes, manifest requires %d", size, expectedSize)
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func removeSQLiteSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func removeSQLiteFiles(path string) {
	_ = os.Remove(path)
	_ = removeSQLiteSidecars(path)
	_ = os.Remove(path + ".lock")
}

func recoverRestoreTarget(ctx context.Context, target, safetyPath string, cause error) error {
	stage, err := makeSiblingTemp(target, ".recover-*.sqlite")
	if err != nil {
		return fmt.Errorf("%w; create recovery stage: %v", cause, err)
	}
	defer removeSQLiteFiles(stage)
	manifest, err := ValidateBackup(ctx, safetyPath)
	if err != nil {
		return fmt.Errorf("%w; validate safety snapshot: %v", cause, err)
	}
	if err := copyFileLimited(ctx, safetyPath, stage, manifest.SizeBytes); err != nil {
		return fmt.Errorf("%w; stage safety snapshot: %v", cause, err)
	}
	if err := removeSQLiteSidecars(target); err != nil {
		return fmt.Errorf("%w; clean target sidecars for recovery: %v", cause, err)
	}
	if err := os.Rename(stage, target); err != nil {
		return fmt.Errorf("%w; restore safety snapshot: %v", cause, err)
	}
	if err := os.Chmod(target, 0o600); err != nil {
		return fmt.Errorf("%w; restrict recovered database: %v", cause, err)
	}
	if err := syncDirectory(filepath.Dir(target)); err != nil {
		return fmt.Errorf("%w; sync recovered database directory: %v", cause, err)
	}
	return fmt.Errorf("%w; current database restored from the safety snapshot", cause)
}
