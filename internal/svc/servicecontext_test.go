package svc

import (
	"context"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/model"
)

func TestNewServiceContextWiring(t *testing.T) {
	db := bootstrapDB(t)
	cfg := config.Config{}
	cfg.Auth = config.AuthConf{
		SessionCookieName:    "relayx_session",
		SessionIdleTTL:       24 * time.Hour,
		SessionAbsoluteTTL:   168 * time.Hour,
		SessionTouchInterval: 5 * time.Minute,
		CookieSecure:         true,
		AllowedOrigins:       []string{"https://panel.example.com"},
		LoginRateLimit:       config.LoginRateLimitConf{Window: time.Minute, MaxAttempts: 10},
	}

	svcCtx := NewServiceContext(cfg, db)
	if svcCtx.UserModel == nil || svcCtx.SessionManager == nil || svcCtx.DB == nil {
		t.Fatalf("service context not wired: %+v", svcCtx)
	}
	if svcCtx.OriginCheck == nil || svcCtx.LoginRateLimit == nil || svcCtx.Auth == nil {
		t.Error("middleware not wired")
	}

	ck := svcCtx.CookieConfig()
	if ck.Name != "relayx_session" || !ck.Secure || ck.MaxAgeSeconds != 604800 {
		t.Errorf("cookie config = %+v", ck)
	}
}

func TestBootstrapAdminFromEnvCreatesAdmin(t *testing.T) {
	db := bootstrapDB(t)
	users := model.NewAuthUserModel(db)

	t.Setenv("RELAYX_ADMIN_ACCOUNT", "admin")
	t.Setenv("RELAYX_ADMIN_PASSWORD", "sup3r-secret-pass")

	res, err := BootstrapAdminFromEnv(context.Background(), db, users)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if !res.Created || res.Account != "admin" {
		t.Fatalf("result = %+v", res)
	}
}
