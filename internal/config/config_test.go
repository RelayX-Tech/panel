package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestLoadPanelConfig(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "etc", "panel-api.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if c.Name != "panel-api" {
		t.Errorf("Name = %q, want panel-api", c.Name)
	}
	if c.Port != 8888 {
		t.Errorf("Port = %d, want 8888", c.Port)
	}
	if c.Auth.SessionCookieName != "relayx_session" {
		t.Errorf("SessionCookieName = %q, want relayx_session", c.Auth.SessionCookieName)
	}
	if c.Auth.SessionIdleTTL != 24*time.Hour {
		t.Errorf("SessionIdleTTL = %v, want 24h", c.Auth.SessionIdleTTL)
	}
	if c.Auth.SessionAbsoluteTTL != 168*time.Hour {
		t.Errorf("SessionAbsoluteTTL = %v, want 168h", c.Auth.SessionAbsoluteTTL)
	}
	if c.Auth.SessionTouchInterval != 5*time.Minute {
		t.Errorf("SessionTouchInterval = %v, want 5m", c.Auth.SessionTouchInterval)
	}
	if c.Auth.LoginRateLimit.Window != 60*time.Second {
		t.Errorf("LoginRateLimit.Window = %v, want 60s", c.Auth.LoginRateLimit.Window)
	}
	if c.Auth.LoginRateLimit.MaxAttempts != 10 {
		t.Errorf("LoginRateLimit.MaxAttempts = %d, want 10", c.Auth.LoginRateLimit.MaxAttempts)
	}
	if c.Auth.PasswordChangeRateLimit.Window != 15*time.Minute {
		t.Errorf("PasswordChangeRateLimit.Window = %v, want 15m", c.Auth.PasswordChangeRateLimit.Window)
	}
	if c.Auth.PasswordChangeRateLimit.MaxAttempts != 5 {
		t.Errorf("PasswordChangeRateLimit.MaxAttempts = %d, want 5", c.Auth.PasswordChangeRateLimit.MaxAttempts)
	}
	if len(c.Auth.AllowedOrigins) == 0 {
		t.Error("AllowedOrigins is empty")
	}
	if len(c.Cache) == 0 {
		t.Error("Cache is empty; password change limiter requires Redis")
	}
}

func TestConfigValidation(t *testing.T) {
	c := Config{
		Auth: AuthConf{
			SessionCookieName:       "relayx_session",
			SessionIdleTTL:          24 * time.Hour,
			SessionAbsoluteTTL:      168 * time.Hour,
			SessionTouchInterval:    5 * time.Minute,
			LoginRateLimit:          LoginRateLimitConf{Window: time.Minute, MaxAttempts: 10},
			PasswordChangeRateLimit: PasswordChangeRateLimitConf{Window: 15 * time.Minute, MaxAttempts: 5},
			AllowedOrigins:          []string{"https://panel.example.com"},
		},
		Cache: cache.CacheConf{{RedisConf: redis.RedisConf{Host: "127.0.0.1:56379", Type: "node"}, Weight: 100}},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	bad := c
	bad.Auth.SessionCookieName = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty SessionCookieName should be rejected")
	}

	bad = c
	bad.Auth.AllowedOrigins = nil
	if err := bad.Validate(); err == nil {
		t.Error("empty AllowedOrigins should be rejected")
	}

	bad = c
	bad.Auth.LoginRateLimit.MaxAttempts = 0
	if err := bad.Validate(); err == nil {
		t.Error("zero MaxAttempts should be rejected")
	}

	bad = c
	bad.Auth.PasswordChangeRateLimit.Window = 0
	if err := bad.Validate(); err == nil {
		t.Error("zero PasswordChangeRateLimit.Window should be rejected")
	}

	bad = c
	bad.Auth.PasswordChangeRateLimit.MaxAttempts = 0
	if err := bad.Validate(); err == nil {
		t.Error("zero PasswordChangeRateLimit.MaxAttempts should be rejected")
	}

	bad = c
	bad.Cache = nil
	if err := bad.Validate(); err == nil {
		t.Error("empty Cache should be rejected (password change limiter requires Redis)")
	}
}
