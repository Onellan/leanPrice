package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	SessionLifetime  = 12 * time.Hour
	MaxLoginAttempts = 5
	LoginWindow      = 15 * time.Minute
	MaxSetupAttempts = 10
)

var (
	ErrNotFound           = errors.New("auth: record not found")
	ErrSetupComplete      = errors.New("auth: initial setup is already complete")
	ErrInvalidSetupToken  = errors.New("auth: setup token is invalid or expired")
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrRateLimited        = errors.New("auth: login attempts are temporarily limited")
	ErrInvalidCSRF        = errors.New("auth: invalid CSRF token")
	usernamePattern       = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,40}$`)
)

type Repository interface {
	HasAdmin(context.Context) (bool, error)
	SetupTokenHash(context.Context) (string, error)
	SetSetupTokenHash(context.Context, string, time.Time) error
	CreateInitialAdmin(context.Context, string, string, string, string, time.Time) error
	AdminByUsername(context.Context, string) (Admin, error)
	CreateSession(context.Context, Session) error
	FindSession(context.Context, string, time.Time) (Session, error)
	TouchSession(context.Context, string, int64, int64) error
	DeleteSession(context.Context, string) error
	ChangeAdminPassword(context.Context, string, string, time.Time) error
}

type Admin struct {
	ID           string
	Username     string
	PasswordHash string
	Enabled      bool
}

type Session struct {
	TokenHash string
	CSRFHash  string
	AdminID   string
	Username  string
	CreatedAt int64
	ExpiresAt int64
	LastSeen  int64
}

type Credentials struct {
	Token string
	CSRF  string
	User  Session
}

type SessionResult struct {
	User      Session
	CSRFHash  string
	Refreshed bool
}

// Service owns first-admin, password and session workflows. The repository
// interface lives here, at the consuming domain boundary.
type Service struct {
	repo         Repository
	hasher       *PasswordHasher
	limiter      *loginLimiter
	setupLimiter *loginLimiter
}

func NewService(repo Repository, hasher *PasswordHasher) (*Service, error) {
	if repo == nil || hasher == nil || hasher.DummyHash() == "" {
		return nil, errors.New("auth: repository and initialized password hasher are required")
	}
	return &Service{repo: repo, hasher: hasher, limiter: &loginLimiter{}, setupLimiter: &loginLimiter{}}, nil
}

func NewSetupToken() (string, error) { return randomToken() }

func TokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func (s *Service) SetupTokenMatches(ctx context.Context, token string) (bool, error) {
	if !s.setupLimiter.AllowLimit(time.Now().UTC(), MaxSetupAttempts) {
		return false, ErrRateLimited
	}
	if len(token) < 40 || len(token) > 64 {
		return false, nil
	}
	stored, err := s.repo.SetupTokenHash(ctx)
	if err != nil {
		return false, err
	}
	if stored == "" {
		return false, nil
	}
	supplied := TokenHash(token)
	return subtle.ConstantTimeCompare([]byte(stored), []byte(supplied)) == 1, nil
}

func (s *Service) SetSetupToken(ctx context.Context, token string, now time.Time) error {
	if len(token) < 40 || len(token) > 64 {
		return errors.New("auth: generated setup token has an invalid length")
	}
	return s.repo.SetSetupTokenHash(ctx, TokenHash(token), now)
}

func (s *Service) HasAdmin(ctx context.Context) (bool, error) {
	return s.repo.HasAdmin(ctx)
}

func (s *Service) CreateInitialAdmin(ctx context.Context, token, username, password string, now time.Time) error {
	if !s.setupLimiter.AllowLimit(now, MaxSetupAttempts) {
		return ErrRateLimited
	}
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return errors.New("username must be 3–40 letters, numbers, dots, dashes or underscores")
	}
	if len(token) < 40 || len(token) > 64 {
		return errors.New("setup token is invalid")
	}
	passwordHash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return err
	}
	adminID, err := randomToken()
	if err != nil {
		return fmt.Errorf("auth: generate administrator ID: %w", err)
	}
	adminID = strings.TrimRight(base64.RawURLEncoding.EncodeToString([]byte(adminID)), "=")
	return s.repo.CreateInitialAdmin(ctx, TokenHash(token), adminID, username, passwordHash, now)
}

func (s *Service) Login(ctx context.Context, username, password, clientKey string, now time.Time) (Credentials, error) {
	if !s.limiter.Allow(now) {
		return Credentials{}, ErrRateLimited
	}
	username = strings.TrimSpace(username)
	if len(username) > 80 || len(password) > 256 || len(clientKey) > 128 {
		_, _ = s.hasher.Verify(ctx, s.hasher.DummyHash(), "invalid login input")
		return Credentials{}, ErrInvalidCredentials
	}
	admin, err := s.repo.AdminByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			_, verifyErr := s.hasher.Verify(ctx, s.hasher.DummyHash(), password)
			if verifyErr != nil {
				return Credentials{}, verifyErr
			}
			return Credentials{}, ErrInvalidCredentials
		}
		return Credentials{}, err
	}
	validPassword, err := s.hasher.Verify(ctx, admin.PasswordHash, password)
	if err != nil {
		return Credentials{}, err
	}
	if !validPassword || !admin.Enabled {
		return Credentials{}, ErrInvalidCredentials
	}
	token, err := randomToken()
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: generate session token: %w", err)
	}
	csrf, err := randomToken()
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: generate CSRF token: %w", err)
	}
	nowMS := now.UTC().UnixMilli()
	session := Session{
		TokenHash: TokenHash(token), CSRFHash: TokenHash(csrf), AdminID: admin.ID, Username: admin.Username,
		CreatedAt: nowMS, ExpiresAt: now.Add(SessionLifetime).UTC().UnixMilli(), LastSeen: nowMS,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return Credentials{}, err
	}
	return Credentials{Token: token, CSRF: csrf, User: session}, nil
}

func (s *Service) CreateSessionForAdmin(ctx context.Context, adminID, username string, now time.Time) (Credentials, error) {
	token, err := randomToken()
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: generate session token: %w", err)
	}
	csrf, err := randomToken()
	if err != nil {
		return Credentials{}, fmt.Errorf("auth: generate CSRF token: %w", err)
	}
	nowMS := now.UTC().UnixMilli()
	session := Session{TokenHash: TokenHash(token), CSRFHash: TokenHash(csrf), AdminID: adminID, Username: username,
		CreatedAt: nowMS, ExpiresAt: now.Add(SessionLifetime).UTC().UnixMilli(), LastSeen: nowMS}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return Credentials{}, err
	}
	return Credentials{Token: token, CSRF: csrf, User: session}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string, now time.Time) (SessionResult, error) {
	if len(token) < 40 || len(token) > 64 {
		return SessionResult{}, ErrInvalidCredentials
	}
	session, err := s.repo.FindSession(ctx, TokenHash(token), now)
	if err != nil {
		return SessionResult{}, err
	}
	result := SessionResult{User: session, CSRFHash: session.CSRFHash}
	if now.Sub(time.UnixMilli(session.LastSeen)) >= 5*time.Minute {
		nowMS := now.UTC().UnixMilli()
		expiresAt := now.Add(SessionLifetime).UTC().UnixMilli()
		if err := s.repo.TouchSession(ctx, session.TokenHash, nowMS, expiresAt); err != nil {
			return SessionResult{}, err
		}
		result.User.LastSeen = nowMS
		result.User.ExpiresAt = expiresAt
		result.Refreshed = true
	}
	return result, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if len(token) < 40 || len(token) > 64 {
		return nil
	}
	return s.repo.DeleteSession(ctx, TokenHash(token))
}

func (s *Service) ChangePassword(ctx context.Context, adminID, username, currentPassword, newPassword string, now time.Time) error {
	admin, err := s.repo.AdminByUsername(ctx, username)
	if err != nil || admin.ID != adminID || !admin.Enabled {
		return ErrInvalidCredentials
	}
	valid, err := s.hasher.Verify(ctx, admin.PasswordHash, currentPassword)
	if err != nil || !valid {
		return ErrInvalidCredentials
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	same, err := s.hasher.Verify(ctx, admin.PasswordHash, newPassword)
	if err != nil {
		return err
	}
	if same {
		return errors.New("new password must be different from the current password")
	}
	passwordHash, err := s.hasher.Hash(ctx, newPassword)
	if err != nil {
		return err
	}
	return s.repo.ChangeAdminPassword(ctx, adminID, passwordHash, now)
}

func ValidCSRF(session SessionResult, cookieToken, submittedToken string) bool {
	if len(cookieToken) < 40 || len(cookieToken) > 64 || len(submittedToken) != len(cookieToken) {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(submittedToken)) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(session.CSRFHash), []byte(TokenHash(cookieToken))) == 1
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

type loginLimiter struct {
	mu      sync.Mutex
	window  time.Time
	attempt int
}

func (l *loginLimiter) Allow(now time.Time) bool {
	return l.AllowLimit(now, MaxLoginAttempts)
}

func (l *loginLimiter) AllowLimit(now time.Time, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.window.IsZero() || now.Sub(l.window) >= LoginWindow || now.Before(l.window) {
		l.window = now
		l.attempt = 0
	}
	if l.attempt >= limit {
		return false
	}
	l.attempt++
	return true
}
