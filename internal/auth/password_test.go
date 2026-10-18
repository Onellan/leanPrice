package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashUsesBoundedArgon2idAndVerifies(t *testing.T) {
	hasher, err := NewPasswordHasher()
	if err != nil {
		t.Fatal(err)
	}
	password := "Correct horse battery staple 2026!"
	encoded, err := hasher.Hash(context.Background(), password)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("password hash header = %q", encoded[:min(40, len(encoded))])
	}
	valid, err := hasher.Verify(context.Background(), encoded, password)
	if err != nil || !valid {
		t.Fatalf("Verify(correct) = %v, %v", valid, err)
	}
	valid, err = hasher.Verify(context.Background(), encoded, "a different password")
	if err != nil || valid {
		t.Fatalf("Verify(wrong) = %v, %v; want false", valid, err)
	}
}

func TestPasswordPolicyRejectsWeakDefaultsAndBoundsInput(t *testing.T) {
	for _, password := range []string{"short", "password1234567", " admin starts and ends with spaces "} {
		if err := ValidatePassword(password); err == nil {
			t.Errorf("ValidatePassword(%q) succeeded", password)
		}
	}
	if err := ValidatePassword("a sufficiently long and unique passphrase"); err != nil {
		t.Fatalf("strong passphrase rejected: %v", err)
	}
}

func TestHashWaitCanBeCanceledAndLoginWindowResets(t *testing.T) {
	hasher, err := NewPasswordHasher()
	if err != nil {
		t.Fatal(err)
	}
	hasher.slot <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hasher.Hash(ctx, "Correct horse battery staple 2026!"); err == nil {
		t.Fatal("Hash() did not return the canceled context while waiting for its slot")
	}
	<-hasher.slot

	limiter := &loginLimiter{}
	base := time.Now().UTC()
	for i := 0; i < MaxLoginAttempts; i++ {
		if !limiter.Allow(base) {
			t.Fatalf("attempt %d was unexpectedly blocked", i+1)
		}
	}
	if limiter.Allow(base) {
		t.Fatal("attempt after limit was allowed")
	}
	if !limiter.Allow(base.Add(LoginWindow)) {
		t.Fatal("limiter did not reset after its window")
	}
}
