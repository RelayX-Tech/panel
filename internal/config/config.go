package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

// Config 是 Panel API 的强类型配置，YAML 键均需有对应字段支撑。
type Config struct {
	rest.RestConf

	Auth     AuthConf     `json:"Auth"`
	Database DatabaseConf `json:"Database,optional"`
}

// AuthConf 管理面认证与会话相关配置（design.md §4.6）。
type AuthConf struct {
	SessionCookieName    string             `json:",default=relayx_session"`
	SessionIdleTTL       time.Duration      `json:",default=24h"`
	SessionAbsoluteTTL   time.Duration      `json:",default=168h"`
	SessionTouchInterval time.Duration      `json:",default=5m"`
	CookieSecure         bool               `json:",default=true"`
	AllowedOrigins       []string           `json:"AllowedOrigins,optional"`
	LoginRateLimit       LoginRateLimitConf `json:"LoginRateLimit"`
	TrustProxyHeaders    bool               `json:",default=false"`
	TrustedProxyCIDRs    []string           `json:",optional"`
}

// LoginRateLimitConf 登录限流：固定窗口内每来源 IP 的最大尝试次数。
type LoginRateLimitConf struct {
	Window      time.Duration `json:",default=60s"`
	MaxAttempts int           `json:",default=10"`
}

// DatabaseConf 连接池配置；DataSource 生产环境经环境变量注入。
type DatabaseConf struct {
	DataSource      string        `json:",optional"`
	MaxOpenConns    int           `json:",default=20"`
	MaxIdleConns    int           `json:",default=10"`
	ConnMaxLifetime time.Duration `json:",default=1h"`
}

// Load 读取 YAML 配置并返回强类型 Config。
func Load(path string) (Config, error) {
	var c Config
	if err := conf.Load(path, &c); err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}
	return c, nil
}

// Validate 校验启动必需配置，避免以不完整配置启动。
func (c Config) Validate() error {
	if c.Auth.SessionCookieName == "" {
		return errors.New("Auth.SessionCookieName is required")
	}
	if len(c.Auth.AllowedOrigins) == 0 {
		return errors.New("Auth.AllowedOrigins is required")
	}
	if c.Auth.SessionIdleTTL <= 0 || c.Auth.SessionAbsoluteTTL <= 0 {
		return errors.New("Auth session TTLs must be positive")
	}
	if c.Auth.SessionAbsoluteTTL < c.Auth.SessionIdleTTL {
		return errors.New("Auth.SessionAbsoluteTTL must not be shorter than SessionIdleTTL")
	}
	if c.Auth.LoginRateLimit.Window <= 0 || c.Auth.LoginRateLimit.MaxAttempts <= 0 {
		return errors.New("Auth.LoginRateLimit window and maxAttempts must be positive")
	}
	return nil
}
