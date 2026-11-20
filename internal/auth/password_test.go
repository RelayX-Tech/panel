package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordProducesArgon2idPHC(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	const prefix = "$argon2id$v=19$m=19456,t=2,p=1$"
	if !strings.HasPrefix(h, prefix) {
		t.Fatalf("hash %q does not start with %q", h, prefix)
	}
	// salt 16 字节 / key 32 字节的 base64 原始编码
	parts := strings.Split(h, "$")
	if len(parts) != 6 {
		t.Fatalf("PHC parts = %d, want 6", len(parts))
	}
	if parts[4] == "" || parts[5] == "" {
		t.Fatal("PHC missing salt or hash segments")
	}
}

func TestHashPasswordUsesUniqueSalt(t *testing.T) {
	a, _ := HashPassword("same-password")
	b, _ := HashPassword("same-password")
	if a == b {
		t.Fatal("two hashes of the same password are identical; salt is not random")
	}
}

func TestVerifyPassword(t *testing.T) {
	h, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	ok, err := VerifyPassword("s3cret-pass", h)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Error("correct password rejected")
	}

	ok, err = VerifyPassword("wrong-pass", h)
	if err != nil {
		t.Fatalf("verify mismatch: %v", err)
	}
	if ok {
		t.Error("wrong password accepted")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "not-a-hash", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=19$m=abc,t=2,p=1$c2FsdA$aGFzaA"} {
		if _, err := VerifyPassword("x", bad); err == nil {
			t.Errorf("malformed hash %q accepted", bad)
		}
	}
}

func TestDummyHashIsValidAndNeverMatches(t *testing.T) {
	if _, err := HashPassword("relayx-dummy"); err != nil {
		t.Fatalf("dummy hash invalid: %v", err)
	}
	if ok, err := VerifyDummy("anything"); err != nil {
		t.Fatalf("verify dummy: %v", err)
	} else if ok {
		t.Error("dummy verification must always fail")
	}
}
