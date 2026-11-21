package config

import (
	"path/filepath"
	"testing"
	"time"
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
	if len(c.Auth.AllowedOrigins) == 0 {
		t.Error("AllowedOrigins is empty")
	}
}

func TestConfigValidation(t *testing.T) {
	c := Config{
		Auth: AuthConf{
			SessionCookieName:    "relayx_session",
			SessionIdleTTL:       24 * time.Hour,
			SessionAbsoluteTTL:   168 * time.Hour,
			SessionTouchInterval: 5 * time.Minute,
			LoginRateLimit:       LoginRateLimitConf{Window: time.Minute, MaxAttempts: 10},
			AllowedOrigins:       []string{"https://panel.example.com"},
		},
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
}
