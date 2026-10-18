package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Onellan/leanPrice/migrations"
)

var migrationName = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

type migration struct {
	version int
	name    string
	sql     string
	hash    string
}

// Migrate applies checked-in forward-only migrations, rejects edited history,
// and refuses databases created by a newer binary.
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("storage: database is not open")
	}
	if ctx == nil {
		return errors.New("storage: context is required")
	}
	files, err := readMigrations()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("storage: no migration files embedded")
	}
	var current int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("storage: read schema version: %w", err)
	}
	if current > files[len(files)-1].version {
		return fmt.Errorf("storage: database schema version %d is newer than this binary supports (latest %d)", current, files[len(files)-1].version)
	}
	if err := s.ensureMigrationTable(ctx); err != nil {
		return err
	}
	applied, err := s.appliedMigrations(ctx)
	if err != nil {
		return err
	}
	known := make(map[int]migration, len(files))
	for _, file := range files {
		known[file.version] = file
	}
	for version, checksum := range applied {
		file, ok := known[version]
		if !ok {
			return fmt.Errorf("storage: database has migration %d unknown to this binary", version)
		}
		if checksum != file.hash {
			return fmt.Errorf("storage: migration %d checksum mismatch", version)
		}
		if version > current {
			return fmt.Errorf("storage: migration %d is recorded above schema version %d", version, current)
		}
	}
	for _, file := range files {
		if file.version <= current {
			if _, ok := applied[file.version]; !ok {
				return fmt.Errorf("storage: schema version %d is missing its migration record", file.version)
			}
			continue
		}
		if file.version != current+1 {
			return fmt.Errorf("storage: migration sequence gap: expected %d, found %d", current+1, file.version)
		}
		if err := s.applyMigration(ctx, file); err != nil {
			return err
		}
		current = file.version
	}
	return nil
}

func readMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return nil, fmt.Errorf("storage: read embedded migrations: %w", err)
	}
	files := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("storage: invalid migration filename %q", entry.Name())
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil || version < 1 {
			return nil, fmt.Errorf("storage: invalid migration version in %q", entry.Name())
		}
		body, err := fs.ReadFile(migrations.Files, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("storage: read migration %s: %w", entry.Name(), err)
		}
		digest := sha256.Sum256(body)
		files = append(files, migration{
			version: version,
			name:    entry.Name(),
			sql:     string(body),
			hash:    hex.EncodeToString(digest[:]),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	for i, file := range files {
		if i > 0 && file.version == files[i-1].version {
			return nil, fmt.Errorf("storage: duplicate migration version %d", file.version)
		}
	}
	return files, nil
}

func (s *Store) ensureMigrationTable(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY CHECK (version > 0),
		name TEXT NOT NULL,
		checksum TEXT NOT NULL CHECK (length(checksum) = 64),
		applied_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("storage: initialize migration tracking: %w", err)
	}
	return nil
}

func (s *Store) appliedMigrations(ctx context.Context) (map[int]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("storage: read migration history: %w", err)
	}
	defer rows.Close()
	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("storage: scan migration history: %w", err)
		}
		if _, exists := applied[version]; exists {
			return nil, fmt.Errorf("storage: duplicate migration history version %d", version)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: read migration history: %w", err)
	}
	return applied, nil
}

func (s *Store) applyMigration(ctx context.Context, file migration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin migration %d: %w", file.version, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, file.sql); err != nil {
		return fmt.Errorf("storage: execute migration %d (%s): %w", file.version, file.name, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES (?, ?, ?, ?)", file.version, file.name, file.hash, time.Now().UTC().UnixMilli()); err != nil {
		return fmt.Errorf("storage: record migration %d: %w", file.version, err)
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(file.version)); err != nil {
		return fmt.Errorf("storage: set schema version %d: %w", file.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit migration %d: %w", file.version, err)
	}
	return nil
}
