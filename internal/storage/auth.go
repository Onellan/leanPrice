package storage

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	authdomain "github.com/Onellan/leanPrice/internal/auth"
)

// HasAdmin reports whether an enabled or disabled administrator exists.
func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM admin_users)").Scan(&exists)
	return exists == 1, err
}

// SetupTokenHash returns the current one-time setup token digest if setup has
// not yet completed.
func (s *Store) SetupTokenHash(ctx context.Context) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, "SELECT setting_value FROM app_settings WHERE setting_key='setup_token_hash'").Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("storage: read setup token state: %w", err)
	}
	return hash, nil
}

// SetSetupTokenHash rotates the bootstrap token only while no administrator
// exists. The caller writes the raw token to a permission-restricted file.
func (s *Store) SetSetupTokenHash(ctx context.Context, hash string, now time.Time) error {
	if len(hash) != 64 {
		return errors.New("storage: setup token digest must be SHA-256 hex")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin setup-token update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM admin_users)").Scan(&exists); err != nil {
		return fmt.Errorf("storage: check setup state: %w", err)
	}
	if exists == 1 {
		return authdomain.ErrSetupComplete
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO app_settings(setting_key, setting_value, updated_at)
		VALUES ('setup_token_hash', ?, ?) ON CONFLICT(setting_key) DO UPDATE SET setting_value=excluded.setting_value, updated_at=excluded.updated_at`, hash, now.UTC().UnixMilli()); err != nil {
		return fmt.Errorf("storage: save setup token digest: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit setup token: %w", err)
	}
	return nil
}

// CreateInitialAdmin consumes the one-time setup token and inserts the first
// administrator plus its audit event atomically.
func (s *Store) CreateInitialAdmin(ctx context.Context, suppliedHash, id, username, passwordHash string, now time.Time) error {
	if len(suppliedHash) != 64 || strings.TrimSpace(id) == "" || strings.TrimSpace(username) == "" || len(strings.TrimSpace(passwordHash)) < 20 {
		return errors.New("storage: initial administrator fields are invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin initial admin creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var storedHash string
	if err := tx.QueryRowContext(ctx, "SELECT setting_value FROM app_settings WHERE setting_key='setup_token_hash'").Scan(&storedHash); err != nil {
		return authdomain.ErrInvalidSetupToken
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(suppliedHash)) != 1 {
		return authdomain.ErrInvalidSetupToken
	}
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM admin_users)").Scan(&exists); err != nil {
		return fmt.Errorf("storage: check initial admin: %w", err)
	}
	if exists == 1 {
		return authdomain.ErrSetupComplete
	}
	nowMS := now.UTC().UnixMilli()
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_users(id, username, password_hash, enabled, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?)`, id, username, passwordHash, nowMS, nowMS); err != nil {
		return fmt.Errorf("storage: insert initial admin: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM app_settings WHERE setting_key='setup_token_hash'"); err != nil {
		return fmt.Errorf("storage: consume setup token: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_audit(admin_user_id, action_code, object_type, object_id, detail_json, created_at)
		VALUES (?, 'admin.initial_setup', 'admin_user', ?, '{}', ?)`, id, id, nowMS); err != nil {
		return fmt.Errorf("storage: audit initial admin: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit initial admin: %w", err)
	}
	return nil
}

func (s *Store) AdminByUsername(ctx context.Context, username string) (authdomain.Admin, error) {
	var admin authdomain.Admin
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, enabled FROM admin_users WHERE username=? COLLATE NOCASE`, strings.TrimSpace(username)).Scan(&admin.ID, &admin.Username, &admin.PasswordHash, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return authdomain.Admin{}, authdomain.ErrNotFound
	}
	if err != nil {
		return authdomain.Admin{}, fmt.Errorf("storage: find admin: %w", err)
	}
	admin.Enabled = enabled == 1
	return admin, nil
}

func (s *Store) CreateSession(ctx context.Context, session authdomain.Session) error {
	if len(session.TokenHash) != 64 || len(session.CSRFHash) != 64 || session.AdminID == "" || session.CreatedAt <= 0 || session.ExpiresAt <= session.CreatedAt {
		return errors.New("storage: session fields are invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin session creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=? OR last_seen_at<=?", session.CreatedAt, session.CreatedAt-12*time.Hour.Milliseconds()); err != nil {
		return fmt.Errorf("storage: remove expired sessions: %w", err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash, admin_user_id, csrf_hash, created_at, expires_at, last_seen_at)
		SELECT ?, id, ?, ?, ?, ? FROM admin_users WHERE id=? AND enabled=1`,
		session.TokenHash, session.CSRFHash, session.CreatedAt, session.ExpiresAt, session.LastSeen, session.AdminID)
	if err != nil {
		return fmt.Errorf("storage: create session: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: session insert result: %w", err)
	}
	if count != 1 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE admin_user_id=? AND token_hash NOT IN
		(SELECT token_hash FROM sessions WHERE admin_user_id=? ORDER BY last_seen_at DESC, created_at DESC LIMIT 16)`, session.AdminID, session.AdminID); err != nil {
		return fmt.Errorf("storage: bound active sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit session: %w", err)
	}
	return nil
}

func (s *Store) FindSession(ctx context.Context, tokenHash string, now time.Time) (authdomain.Session, error) {
	var session authdomain.Session
	err := s.db.QueryRowContext(ctx, `SELECT s.token_hash, s.csrf_hash, s.admin_user_id, u.username, s.created_at, s.expires_at, s.last_seen_at
		FROM sessions s JOIN admin_users u ON u.id=s.admin_user_id
		WHERE s.token_hash=? AND u.enabled=1 AND s.expires_at>? AND s.last_seen_at>?`,
		tokenHash, now.UTC().UnixMilli(), now.Add(-12*time.Hour).UTC().UnixMilli()).Scan(
		&session.TokenHash, &session.CSRFHash, &session.AdminID, &session.Username, &session.CreatedAt, &session.ExpiresAt, &session.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return authdomain.Session{}, authdomain.ErrNotFound
	}
	if err != nil {
		return authdomain.Session{}, fmt.Errorf("storage: find session: %w", err)
	}
	return session, nil
}

func (s *Store) TouchSession(ctx context.Context, tokenHash string, lastSeen, expiresAt int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at=?, expires_at=? WHERE token_hash=?", lastSeen, expiresAt, tokenHash)
	if err != nil {
		return fmt.Errorf("storage: refresh session: %w", err)
	}
	return nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", tokenHash); err != nil {
		return fmt.Errorf("storage: delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteAdminSessions(ctx context.Context, adminID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE admin_user_id=?", adminID); err != nil {
		return fmt.Errorf("storage: invalidate admin sessions: %w", err)
	}
	return nil
}

func (s *Store) ChangeAdminPassword(ctx context.Context, adminID, passwordHash string, now time.Time) error {
	if strings.TrimSpace(adminID) == "" || len(passwordHash) < 20 {
		return errors.New("storage: admin ID and password hash are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin password change: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	nowMS := now.UTC().UnixMilli()
	result, err := tx.ExecContext(ctx, "UPDATE admin_users SET password_hash=?, updated_at=? WHERE id=?", passwordHash, nowMS, adminID)
	if err != nil {
		return fmt.Errorf("storage: change admin password: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE admin_user_id=?", adminID); err != nil {
		return fmt.Errorf("storage: revoke sessions after password change: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_audit(admin_user_id, action_code, object_type, object_id, detail_json, created_at)
		VALUES (?, 'admin.password_changed', 'admin_user', ?, '{}', ?)`, adminID, adminID, nowMS); err != nil {
		return fmt.Errorf("storage: audit password change: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit password change: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=? OR last_seen_at<=?", now.UTC().UnixMilli(), now.Add(-12*time.Hour).UTC().UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("storage: clean expired sessions: %w", err)
	}
	count, err := result.RowsAffected()
	return count, err
}
