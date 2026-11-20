package auth

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"
)

func TestGenerateSessionToken(t *testing.T) {
	tok, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(tok) != 43 {
		t.Fatalf("token length = %d, want 43 (base64url of 32 bytes)", len(tok))
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		t.Fatalf("token is not base64url: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("decoded length = %d, want 32", len(raw))
	}

	other, _ := GenerateSessionToken()
	if tok == other {
		t.Fatal("two session tokens are identical")
	}
}

func TestHashSessionTokenIsSHA256(t *testing.T) {
	tok, _ := GenerateSessionToken()
	h := HashSessionToken(tok)
	if len(h) != 32 {
		t.Fatalf("hash length = %d, want 32", len(h))
	}
	if !bytes.Equal(h, HashSessionToken(tok)) {
		t.Fatal("hash is not deterministic")
	}
	other, _ := GenerateSessionToken()
	if bytes.Equal(h, HashSessionToken(other)) {
		t.Fatal("different tokens produced the same hash")
	}
}

func TestNewSessionExpiry(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	created := now
	idle, absolute := 24*time.Hour, 168*time.Hour

	expires, abs := NewSessionExpiry(now, created, idle, absolute)
	if !expires.Equal(now.Add(idle)) {
		t.Errorf("expiresAt = %v, want %v", expires, now.Add(idle))
	}
	if !abs.Equal(created.Add(absolute)) {
		t.Errorf("absoluteExpiresAt = %v, want %v", abs, created.Add(absolute))
	}

	// 接近绝对上限时，空闲到期不得突破绝对上限
	late := created.Add(absolute - time.Hour)
	expires, abs = NewSessionExpiry(late, created, idle, absolute)
	if !expires.Equal(created.Add(absolute)) {
		t.Errorf("expiresAt = %v, want capped at absolute %v", expires, created.Add(absolute))
	}
	if !abs.Equal(created.Add(absolute)) {
		t.Errorf("absoluteExpiresAt = %v, want %v", abs, created.Add(absolute))
	}
}

func TestShouldTouch(t *testing.T) {
	now := time.Now()
	interval := 5 * time.Minute

	if ShouldTouch(now.Add(-4*time.Minute-59*time.Second), now, interval) {
		t.Error("touched before 5m interval")
	}
	if !ShouldTouch(now.Add(-5*time.Minute), now, interval) {
		t.Error("not touched at exactly 5m")
	}
	if !ShouldTouch(now.Add(-5*time.Minute-time.Second), now, interval) {
		t.Error("not touched after 5m interval")
	}
}

func TestSessionValidity(t *testing.T) {
	now := time.Now()
	base := SessionState{
		ExpiresAt:         now.Add(time.Hour),
		AbsoluteExpiresAt: now.Add(24 * time.Hour),
	}

	if st := base.Evaluate(now); st != SessionValid {
		t.Errorf("valid session evaluated as %v", st)
	}

	expired := base
	expired.ExpiresAt = now.Add(-time.Second)
	if st := expired.Evaluate(now); st != SessionExpiredState {
		t.Errorf("idle-expired session evaluated as %v", st)
	}

	revoked := base
	revoked.RevokedAt = now
	if st := revoked.Evaluate(now); st != SessionExpiredState {
		t.Errorf("revoked session evaluated as %v", st)
	}

	absolute := base
	absolute.AbsoluteExpiresAt = now.Add(-time.Second)
	if st := absolute.Evaluate(now); st != SessionExpiredState {
		t.Errorf("absolute-expired session evaluated as %v", st)
	}

	disabled := base
	disabled.UserDisabled = true
	if st := disabled.Evaluate(now); st != SessionExpiredState {
		t.Errorf("disabled-user session evaluated as %v", st)
	}
}
