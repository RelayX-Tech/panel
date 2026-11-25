// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package svc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/relayx-tech/panel/internal/auth"
	agentclient "github.com/relayx-tech/panel/internal/client/agent"
	"github.com/relayx-tech/panel/internal/client/corecontrol"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/cryptobox"
	"github.com/relayx-tech/panel/internal/logic/dispatch"
	"github.com/relayx-tech/panel/internal/logic/register"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/notify"
	"github.com/relayx-tech/panel/internal/orchestrator"
	"github.com/relayx-tech/panel/internal/pki"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/system"
	"github.com/relayx-tech/panel/internal/upgrade"
	"github.com/relayx-tech/panel/internal/ws"
)

// TransactionRunner 是 logic 依赖的事务边界（便于单测注入假实现）。
type TransactionRunner interface {
	TransactCtx(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error
}

// ServiceContext 是组合根：配置、model、会话管理与中间件在启动时构造一次。
// RedisPinger 是健康检查所需的最小 Redis 能力。
type RedisPinger interface {
	Ping() bool
}

type ServiceContext struct {
	Config          config.Config
	DB              sqlx.SqlConn
	Redis           RedisPinger
	Tx              TransactionRunner
	UserModel       model.AuthUserModel
	SessionModel    model.AuthSessionModel
	AuditModel      model.AuthPasswordAuditModel
	SessionManager  *auth.SessionManager
	PasswordLimiter auth.PasswordLimiterStore

	OriginCheck    rest.Middleware
	LoginRateLimit rest.Middleware
	Auth           rest.Middleware

	// STO-002 下发编排客户端（未配置端点时为 nil）。
	CoreControl *corecontrol.Client
	Agent       *agentclient.Client

	// STO-004 秘密字段加密（RELAYX_MASTER_KEY 缺失时为 nil，写接口会拒绝）。
	Box *cryptobox.Box

	// STO-002 运行时编排 / 注册（未配置时为 nil；供内部触发与 gRPC 服务使用）。
	Register     *register.Service
	Dispatch     *dispatch.Service
	AuditSink    dispatch.AuditSink
	Orchestrator *orchestrator.Orchestrator

	// STO-004 读模型与设置（design.md §5.1）。
	NodeModel          model.NodeModel
	AlertModel         model.AlertModel
	TrafficModel       model.TrafficModel
	ClientModel        model.ClientModel
	SettingModel       model.SettingModel
	SubscriptionModel  model.SubscriptionModel
	NotificationModel  model.NotificationModel
	GeoModel           model.GeoModel
	AuditLogModel      model.AuditLogModel
	UpgradeModel       model.UpgradeModel
	SettingsApplyModel model.SettingsApplyModel
	Upgrade            *upgrade.Service
	WSHub              *ws.Hub
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

	// 健康检查（AC85）需要独立的 Redis 连通性探测。
	var redisPinger RedisPinger
	if len(cacheConf) > 0 {
		redisPinger = cacheConf[0].RedisConf.NewRedis()
	}

	coreControl, agentClient := dialDispatchClients(c)

	dispatchSvc, auditSink, box := buildDispatch(c, conn)
	registerSvc := buildRegister(c, conn)
	var orch *orchestrator.Orchestrator
	if agentClient != nil && dispatchSvc != nil {
		applier := orchestrator.NewAgentApplier(func(string) (orchestrator.AgentClient, error) {
			return agentClient, nil
		})
		orch = orchestrator.New(applier)
	}

	return &ServiceContext{
		Config:             c,
		DB:                 conn,
		Redis:              redisPinger,
		Tx:                 conn,
		UserModel:          users,
		SessionModel:       sessions,
		AuditModel:         audits,
		SessionManager:     manager,
		PasswordLimiter:    passwordLimiter,
		OriginCheck:        middleware.NewOriginCheckMiddleware(c.Auth.AllowedOrigins).Handle,
		LoginRateLimit:     rateLimiter.Handle,
		Auth:               middleware.NewAuthMiddleware(manager, c.Auth.SessionCookieName).Handle,
		CoreControl:        coreControl,
		Agent:              agentClient,
		Register:           registerSvc,
		Dispatch:           dispatchSvc,
		AuditSink:          auditSink,
		Orchestrator:       orch,
		Box:                box,
		NodeModel:          model.NewNodeModel(conn),
		AlertModel:         model.NewAlertModel(conn),
		TrafficModel:       model.NewTrafficModel(conn),
		ClientModel:        model.NewClientModel(conn),
		SettingModel:       model.NewSettingModel(conn),
		SubscriptionModel:  model.NewSubscriptionModel(conn),
		NotificationModel:  model.NewNotificationModel(conn),
		GeoModel:           model.NewGeoModel(conn),
		AuditLogModel:      model.NewAuditLogModel(conn),
		UpgradeModel:       model.NewUpgradeModel(conn),
		SettingsApplyModel: model.NewSettingsApplyModel(conn),
		Upgrade:            buildUpgradeService(c, conn, agentClient, box),
		WSHub:              buildWSHub(cacheConf),
	}
}

// buildWSHub 组装 WS Hub；Redis 可用时启用多实例扇出（design.md §5.2）。
func buildWSHub(cacheConf cache.CacheConf) *ws.Hub {
	var fanout ws.Fanout
	if len(cacheConf) > 0 {
		fanout = ws.NewRedisFanout(cacheConf[0].Host, cacheConf[0].Pass)
	}
	return ws.NewHub(fanout)
}

// buildUpgradeService 组装升级编排（design.md §5.5）。
func buildUpgradeService(c config.Config, conn sqlx.SqlConn, agentClient *agentclient.Client, box *cryptobox.Box) *upgrade.Service {
	svc := &upgrade.Service{
		Model:        model.NewUpgradeModel(conn),
		Nodes:        model.NewNodeModel(conn),
		Alerts:       model.NewAlertModel(conn),
		Holder:       fmt.Sprintf("panel/%s-%d", hostnameOr("unknown"), os.Getpid()),
		PanelVersion: system.Version,
		Channel:      c.Update.Channel,
		ComposeMode:  c.Deployment.Mode == "compose",
		SchemaEqual:  true,
		PanelMinor:   contractMinor(system.ContractVersion),
		AgentMinor:   contractMinor(system.ContractVersion),
		Async:        true,
	}
	if agentClient != nil {
		svc.Agents = &upgrade.GRPCAgentUpdater{Client: agentClient}
	}
	if c.Upgrader.Enabled {
		if client, err := upgrade.NewUpgraderHTTP(c.Upgrader.SocketPath, c.Upgrader.TokenFile, c.Upgrader.Timeout); err == nil {
			svc.Upgrader = client
		}
	}
	if c.Update.SourceURL != "" {
		svc.Source = &upgrade.HTTPManifestSource{ManifestURL: c.Update.SourceURL, PublicKeys: c.Update.PublicKeys}
	}
	if box != nil {
		svc.Notifier = &notify.Dispatcher{
			Channels: model.NewNotificationModel(conn),
			Alerts:   model.NewAlertModel(conn),
			Box:      box,
			Sender:   notify.NewClient(),
		}
	}
	return svc
}

func hostnameOr(fallback string) string {
	if name, err := os.Hostname(); err == nil && name != "" {
		return name
	}
	return fallback
}

// contractMinor 取 "1.4.0" 的 minor 段。
func contractMinor(version string) string {
	parts := make([]string, 0, 3)
	for _, p := range strings.Split(version, ".") {
		parts = append(parts, p)
	}
	if len(parts) >= 2 {
		if _, err := strconv.Atoi(parts[1]); err == nil {
			return parts[1]
		}
	}
	return "0"
}
func buildDispatch(_ config.Config, conn sqlx.SqlConn) (*dispatch.Service, dispatch.AuditSink, *cryptobox.Box) {
	auditSink := dispatch.NewSQLAuditSink(conn)
	store := dispatch.NewSQLStore(conn)
	masterKey := os.Getenv("RELAYX_MASTER_KEY")
	if masterKey == "" {
		return nil, auditSink, nil
	}
	key, err := base64.StdEncoding.DecodeString(masterKey)
	if err != nil {
		return nil, auditSink, nil
	}
	box, err := cryptobox.New(key, "")
	if err != nil {
		return nil, auditSink, nil
	}
	return dispatch.New(store, box), auditSink, box
}

// buildRegister 构造注册服务（需配置 Panel CA）。
func buildRegister(c config.Config, conn sqlx.SqlConn) *register.Service {
	if c.Registration.CACertFile == "" || c.Registration.CAKeyFile == "" {
		return nil
	}
	ca, err := pki.LoadCA(c.Registration.CACertFile, c.Registration.CAKeyFile)
	if err != nil {
		return nil
	}
	return register.New(register.NewSQLStore(conn), ca)
}

// dialDispatchClients 按配置建立 core-control / Agent 客户端；未配置端点返回 nil。
func dialDispatchClients(c config.Config) (*corecontrol.Client, *agentclient.Client) {
	var coreControl *corecontrol.Client
	if len(c.CoreControl.Endpoints) > 0 {
		if opts, err := dialOptions(c.CoreControl.Insecure, c.CoreControl.CertFile, c.CoreControl.KeyFile, c.CoreControl.CAFile, c.CoreControl.ServerName); err == nil {
			if client, derr := corecontrol.Dial(c.CoreControl.Endpoints[0], opts...); derr == nil {
				coreControl = client
			}
		}
	}
	var agentClient *agentclient.Client
	if len(c.Agent.Endpoints) > 0 {
		if opts, err := dialOptions(c.Agent.Insecure, c.Agent.CertFile, c.Agent.KeyFile, c.Agent.CAFile, c.Agent.ServerName); err == nil {
			if client, derr := agentclient.Dial(c.Agent.Endpoints[0], opts...); derr == nil {
				agentClient = client
			}
		}
	}
	return coreControl, agentClient
}

func dialOptions(insecureMode bool, certFile, keyFile, caFile, serverName string) ([]grpc.DialOption, error) {
	if insecureMode {
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, ServerName: serverName}
	if certFile != "" && keyFile != "" && caFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("CA PEM 解析失败: %s", caFile)
		}
		cfg.Certificates = []tls.Certificate{cert}
		cfg.RootCAs = pool
	}
	return []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(cfg))}, nil
}

// CookieConfig 返回会话 Cookie 的传输配置。
func (s *ServiceContext) CookieConfig() response.CookieConfig {
	return response.CookieConfig{
		Name:          s.Config.Auth.SessionCookieName,
		Secure:        s.Config.Auth.CookieSecure,
		MaxAgeSeconds: int(s.Config.Auth.SessionAbsoluteTTL.Seconds()),
	}
}
