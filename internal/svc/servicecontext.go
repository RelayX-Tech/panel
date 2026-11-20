// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package svc

import (
	"database/sql"

	"github.com/zeromicro/go-zero/rest"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
)

// ServiceContext 是组合根：配置、model、会话管理与中间件在启动时构造一次。
type ServiceContext struct {
	Config         config.Config
	DB             *sql.DB
	UserModel      model.AuthUserModel
	SessionManager *auth.SessionManager

	OriginCheck    rest.Middleware
	LoginRateLimit rest.Middleware
	Auth           rest.Middleware
}

func NewServiceContext(c config.Config, db *sql.DB) *ServiceContext {
	users := model.NewAuthUserModel(db)
	sessions := model.NewAuthSessionModel(db)
	manager := auth.NewSessionManager(sessions, c.Auth.SessionIdleTTL, c.Auth.SessionAbsoluteTTL, c.Auth.SessionTouchInterval)

	rateLimiter := middleware.NewLoginRateLimitMiddleware(
		c.Auth.LoginRateLimit.Window, c.Auth.LoginRateLimit.MaxAttempts, "/api/v1/auth/login")
	rateLimiter.TrustProxy = c.Auth.TrustProxyHeaders
	rateLimiter.TrustedProxyCIDRs = c.Auth.TrustedProxyCIDRs

	return &ServiceContext{
		Config:         c,
		DB:             db,
		UserModel:      users,
		SessionManager: manager,
		OriginCheck:    middleware.NewOriginCheckMiddleware(c.Auth.AllowedOrigins).Handle,
		LoginRateLimit: rateLimiter.Handle,
		Auth:           middleware.NewAuthMiddleware(manager, c.Auth.SessionCookieName).Handle,
	}
}

// CookieConfig 返回会话 Cookie 的传输配置。
func (s *ServiceContext) CookieConfig() response.CookieConfig {
	return response.CookieConfig{
		Name:          s.Config.Auth.SessionCookieName,
		Secure:        s.Config.Auth.CookieSecure,
		MaxAgeSeconds: int(s.Config.Auth.SessionAbsoluteTTL.Seconds()),
	}
}
