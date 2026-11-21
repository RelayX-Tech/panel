// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package svc

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
)

// TransactionRunner 是 logic 依赖的事务边界（便于单测注入假实现）。
type TransactionRunner interface {
	TransactCtx(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error
}

// ServiceContext 是组合根：配置、model、会话管理与中间件在启动时构造一次。
type ServiceContext struct {
	Config          config.Config
	DB              sqlx.SqlConn
	Tx              TransactionRunner
	UserModel       model.AuthUserModel
	SessionModel    model.AuthSessionModel
	AuditModel      model.AuthPasswordAuditModel
	SessionManager  *auth.SessionManager
	PasswordLimiter auth.PasswordLimiterStore

	OriginCheck    rest.Middleware
	LoginRateLimit rest.Middleware
	Auth           rest.Middleware
}

func NewServiceContext(c config.Config, conn sqlx.SqlConn, cacheConf cache.CacheConf) *ServiceContext {
	users := model.NewAuthUserModel(conn, cacheConf)
	sessions := model.NewAuthSessionModel(conn, cacheConf, users)
	audits := model.NewAuthPasswordAuditModel(conn)
	manager := auth.NewSessionManager(sessions, c.Auth.SessionIdleTTL, c.Auth.SessionAbsoluteTTL, c.Auth.SessionTouchInterval)

	rateLimiter := middleware.NewLoginRateLimitMiddleware(
		c.Auth.LoginRateLimit.Window, c.Auth.LoginRateLimit.MaxAttempts, "/api/v1/auth/login")
	rateLimiter.TrustProxy = c.Auth.TrustProxyHeaders
	rateLimiter.TrustedProxyCIDRs = c.Auth.TrustedProxyCIDRs

	// 修改密码限流复用 Cache（Redis）连接；Cache 缺失由 Config.Validate 拒绝启动。
	var passwordLimiter auth.PasswordLimiterStore
	if len(cacheConf) > 0 {
		passwordLimiter = auth.NewPasswordLimiter(
			cacheConf[0].RedisConf.NewRedis(),
			c.Auth.PasswordChangeRateLimit.Window,
			c.Auth.PasswordChangeRateLimit.MaxAttempts,
		)
	}

	return &ServiceContext{
		Config:          c,
		DB:              conn,
		Tx:              conn,
		UserModel:       users,
		SessionModel:    sessions,
		AuditModel:      audits,
		SessionManager:  manager,
		PasswordLimiter: passwordLimiter,
		OriginCheck:     middleware.NewOriginCheckMiddleware(c.Auth.AllowedOrigins).Handle,
		LoginRateLimit:  rateLimiter.Handle,
		Auth:            middleware.NewAuthMiddleware(manager, c.Auth.SessionCookieName).Handle,
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
