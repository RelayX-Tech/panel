package observability

import (
	"strings"
	"testing"
)

func TestUserHashIsStableAndNormalized(t *testing.T) {
	a := UserHash("Admin")
	b := UserHash("  admin  ")

	if a == "" {
		t.Fatal("UserHash returned empty string")
	}
	if a != b {
		t.Fatalf("UserHash is not trim/case normalized: %q != %q", a, b)
	}
	if len(a) < 16 {
		t.Errorf("UserHash = %q, want at least 16 hex characters", a)
	}
	if strings.Contains(strings.ToLower(a), "admin") {
		t.Errorf("UserHash leaks account plaintext: %q", a)
	}
}

func TestUserHashDiffersByAccount(t *testing.T) {
	if UserHash("admin") == UserHash("guest") {
		t.Fatal("different accounts produced the same userHash")
	}
}
