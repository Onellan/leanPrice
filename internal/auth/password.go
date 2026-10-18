package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 2
	argonMemory  uint32 = 19 * 1024 // KiB; OWASP's minimum Argon2id profile.
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	saltLength          = 16
)

// PasswordHasher bounds memory use by allowing at most one Argon2 operation
// per process. Requests waiting for the slot remain cancelable.
type PasswordHasher struct {
	slot      chan struct{}
	dummyHash string
}

func NewPasswordHasher() (*PasswordHasher, error) {
	hasher := &PasswordHasher{slot: make(chan struct{}, 1)}
	salt := []byte("leanPrice-dummy!")
	hash := argon2.IDKey([]byte("not-a-real-leanprice-account"), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	hasher.dummyHash = encodeHash(salt, hash)
	clear(hash)
	return hasher, nil
}

// Hash returns an encoded Argon2id hash after applying the setup password
// policy. The exact cost tuple is fixed so a stored hash cannot request
// attacker-controlled memory or CPU at login.
func (h *PasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate password salt: %w", err)
	}
	passwordBytes := []byte(password)
	derived := argon2.IDKey(passwordBytes, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	clear(passwordBytes)
	encoded := encodeHash(salt, derived)
	clear(derived)
	return encoded, nil
}

// Verify checks an encoded hash in constant time. Malformed hashes are
// rejected after one dummy derivation, avoiding a fast path for unknown users.
func (h *PasswordHasher) Verify(ctx context.Context, encoded, password string) (bool, error) {
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()
	salt, expected, ok := parseHash(encoded)
	if !ok {
		salt, expected, _ = parseHash(h.dummyHash)
	}
	passwordBytes := []byte(password)
	actual := argon2.IDKey(passwordBytes, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	clear(passwordBytes)
	valid := ok && subtle.ConstantTimeCompare(actual, expected) == 1
	clear(actual)
	clear(expected)
	return valid, nil
}

func (h *PasswordHasher) DummyHash() string {
	if h == nil {
		return ""
	}
	return h.dummyHash
}

func (h *PasswordHasher) acquire(ctx context.Context) error {
	if h == nil || h.slot == nil {
		return fmt.Errorf("auth: password hasher is not initialized")
	}
	if ctx == nil {
		return fmt.Errorf("auth: context is required")
	}
	select {
	case h.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *PasswordHasher) release() { <-h.slot }

func encodeHash(salt, hash []byte) string {
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
}

func parseHash(encoded string) ([]byte, []byte, bool) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return nil, nil, false
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return nil, nil, false
	}
	values := make(map[string]uint64, 3)
	for _, param := range params {
		key, value, ok := strings.Cut(param, "=")
		if !ok {
			return nil, nil, false
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, nil, false
		}
		values[key] = parsed
	}
	if values["m"] != uint64(argonMemory) || values["t"] != uint64(argonTime) || values["p"] != uint64(argonThreads) {
		return nil, nil, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != saltLength {
		return nil, nil, false
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) != int(argonKeyLen) {
		return nil, nil, false
	}
	return salt, hash, true
}

// ValidatePassword rejects short or known default/common first-run passwords.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || len(password) < 14 || len(password) > 128 {
		return fmt.Errorf("password must be 14–128 UTF-8 bytes")
	}
	if strings.TrimSpace(password) != password {
		return fmt.Errorf("password cannot start or end with whitespace")
	}
	switch strings.ToLower(password) {
	case "password123456", "password1234567", "administrator", "administrator1", "leanprice12345", "changeme123456", "qwertyuiopasdf", "letmein1234567", "welcome1234567", "adminadmin1234":
		return fmt.Errorf("choose a password that is not a common default")
	}
	return nil
}
