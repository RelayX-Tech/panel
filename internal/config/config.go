package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/rest"
)

// Config 是 Panel API 的强类型配置，YAML 键均需有对应字段支撑。
type Config struct {
	rest.RestConf

	Auth     AuthConf        `json:"Auth"`
	Database DatabaseConf    `json:"Database,optional"`
	Cache    cache.CacheConf `json:"Cache,optional"`

	// STO-002 下发编排：core-control / Agent gRPC 客户端。
	CoreControl CoreControlConf `json:"CoreControl,optional"`
	Agent       AgentConf       `json:"Agent,optional"`

	// STO-002 节点注册：RegistrationService gRPC 服务端 + Panel CA。
	Registration RegistrationConf `json:"Registration,optional"`

	// STO-004 部署形态与升级外挂（design.md §7.3）。
	Deployment DeploymentConf `json:"Deployment,optional"`
	Upgrader   UpgraderConf   `json:"Upgrader,optional"`

	// STO-004 对外访问地址（订阅预览等使用）。
	PublicURL string `json:",optional"`

	// STO-004 Geo 文件目录。
	Geo GeoConf `json:"Geo,optional"`

	// STO-004 在线升级更新源。
	Update UpdateConf `json:"Update,optional"`

	// STO-004 订阅格式能力（来自 core 的 capabilities；为空时以 DB 为准）。
	Subscription SubscriptionConf `json:"Subscription,optional"`
}

// SubscriptionConf 配置后端支持的订阅格式能力。
type SubscriptionConf struct {
	Capabilities []string `json:",optional"`
}

// UpdateConf 配置更新源与验签公钥（design.md §5.6）。
type UpdateConf struct {
	SourceURL  string   `json:",optional"`
	Channel    string   `json:",default=stable"`
	PublicKeys []string `json:",optional"`
}

// GeoConf 配置 Geo 文件存放与更新源。
type GeoConf struct {
	Dir             string `json:",default=/var/lib/relayx/geo"`
	UpdateSourceURL string `json:",optional"`
}

// DeploymentConf 描述部署形态：binary（systemd）或 compose。
type DeploymentConf struct {
	Mode string `json:",default=binary"` // binary | compose
}

// UpgraderConf 配置 relayx-upgrader 本地 UDS 客户端。
type UpgraderConf struct {
	Enabled    bool          `json:",default=false"`
	SocketPath string        `json:",default=/run/relayx/upgrader.sock"`
	TokenFile  string        `json:",default=/etc/relayx/upgrader.token"`
	Timeout    time.Duration `json:",default=30m"`
}

// RegistrationConf 配置 RegistrationService gRPC 监听与 Panel CA（design §3.10）。
type RegistrationConf struct {
	Enabled    bool   `json:",default=false"`
	ListenOn   string `json:",default=127.0.0.1:9101"`
	Insecure   bool   `json:",default=true"`
	CertFile   string `json:",optional"` // 服务端证书（server-auth TLS）
	KeyFile    string `json:",optional"`
	CAFile     string `json:",optional"` // 校验客户端证书（Renew）
	CACertFile string `json:",optional"` // Panel CA 证书（签发）
	CAKeyFile  string `json:",optional"` // Panel CA 私钥（签发）
}

// CoreControlConf 配置 core-control gRPC 客户端（随 Panel 部署，回环 / 内网）。
type CoreControlConf struct {
	Endpoints  []string      `json:",optional"`
	Timeout    time.Duration `json:",default=35s"`
	Insecure   bool          `json:",default=true"`
	CertFile   string        `json:",optional"`
	KeyFile    string        `json:",optional"`
	CAFile     string        `json:",optional"`
	ServerName string        `json:",optional"`
}

// AgentConf 配置 Agent gRPC 客户端（每节点一个）。
type AgentConf struct {
	Endpoints  []string      `json:",optional"`
	Timeout    time.Duration `json:",default=35s"`
	Insecure   bool          `json:",default=true"`
	CertFile   string        `json:",optional"`
	KeyFile    string        `json:",optional"`
	CAFile     string        `json:",optional"`
	ServerName string        `json:",optional"`
}

// AuthConf 管理面认证与会话相关配置（design.md §4.6）。
type AuthConf struct {
	SessionCookieName       string                      `json:",default=relayx_session"`
	SessionIdleTTL          time.Duration               `json:",default=24h"`
	SessionAbsoluteTTL      time.Duration               `json:",default=168h"`
	SessionTouchInterval    time.Duration               `json:",default=5m"`
	CookieSecure            bool                        `json:",default=true"`
	AllowedOrigins          []string                    `json:"AllowedOrigins,optional"`
	LoginRateLimit          LoginRateLimitConf          `json:"LoginRateLimit"`
	PasswordChangeRateLimit PasswordChangeRateLimitConf `json:"PasswordChangeRateLimit"`
	TrustProxyHeaders       bool                        `json:",default=false"`
	TrustedProxyCIDRs       []string                    `json:",optional"`
}

// LoginRateLimitConf 登录限流：固定窗口内每来源 IP 的最大尝试次数。
type LoginRateLimitConf struct {
	Window      time.Duration `json:",default=60s"`
	MaxAttempts int           `json:",default=10"`
}

// PasswordChangeRateLimitConf 修改密码旧密码错误限流：按用户 ID 的固定窗口，
// 与登录限流完全独立（design.md §4.2 / AC23 / AC24 / AC42）。
type PasswordChangeRateLimitConf struct {
	Window      time.Duration `json:",default=15m"`
	MaxAttempts int           `json:",default=5"`
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
	if c.Auth.PasswordChangeRateLimit.Window <= 0 || c.Auth.PasswordChangeRateLimit.MaxAttempts <= 0 {
		return errors.New("Auth.PasswordChangeRateLimit window and maxAttempts must be positive")
	}
	// 修改密码限流与 sqlcache 共用 Redis；缺失时无法保证 AC23/AC24/AC42。
	if len(c.Cache) == 0 {
		return errors.New("Cache (Redis) is required for password change rate limiting")
	}
	return nil
}
