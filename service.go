package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/handler"
	"github.com/relayx-tech/panel/internal/health"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/server/registration"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/migrations"
)

const (
	sessionRetention = 7 * 24 * time.Hour
	auditRetention   = 180 * 24 * time.Hour
	cleanupInterval  = time.Hour
	startupTimeout   = 30 * time.Second
)

var configFile = flag.String("f", "etc/panel-api.yaml", "the config file")

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		os.Exit(runMigrate(os.Args[2:]))
	}
	flag.Parse()

	c, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := c.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	db, err := openDatabase(c)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	// 业务连接复用同一连接池（design.md §5.1.3）：openDatabase 保留给迁移 / bootstrap 例外。
	conn := sqlx.NewSqlConnFromDB(db)

	startupCtx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	if err := migrate.Check(startupCtx, db, migrations.FS); err != nil {
		log.Fatalf("schema not ready: %v (run `panel migrate up`)", err)
	}

	res, err := svc.BootstrapAdminFromEnv(startupCtx, db, model.NewAuthUserModel(conn, c.Cache))
	if err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}
	if res.Created {
		logx.Infow("initial admin created", logx.Field("account", res.Account))
		logx.Info("please unset RELAYX_ADMIN_PASSWORD after first startup")
	}
	if res.Warning != "" {
		logx.Errorw("admin bootstrap warning", logx.Field("warning", res.Warning))
	}

	response.InstallErrorHandler()

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// 链路顺序：Recovery → RequestID → AccessLog → SecurityHeaders → (路由级) OriginCheck → Auth → LoginRateLimit
	server.Use(middleware.Recovery())
	server.Use(requestid.Middleware())
	server.Use(middleware.AccessLog())
	server.Use(middleware.SecurityHeaders())

	svcCtx := svc.NewServiceContext(c, conn, c.Cache)

	// 节点注册 gRPC 服务（design §3.10）；启用且已配置 Panel CA 时启动。
	if c.Registration.Enabled && svcCtx.Register != nil {
		regSrv, err := registration.NewGRPCServer(c.Registration, svcCtx.Register)
		if err != nil {
			log.Fatalf("registration server init: %v", err)
		}
		lis, err := net.Listen("tcp", c.Registration.ListenOn)
		if err != nil {
			log.Fatalf("registration listen: %v", err)
		}
		go func() {
			if serr := regSrv.Serve(lis); serr != nil {
				logx.Errorw("registration gRPC stopped", logx.Field("error", serr.Error()))
			}
		}()
		proc.AddShutdownListener(func() { regSrv.GracefulStop() })
		logx.Infow("registration gRPC listening", logx.Field("addr", c.Registration.ListenOn))
	}

	// readyz：DB / 迁移 /（已配置时）core-control 与 Agent 可达性。
	readyzChecks := []func() error{
		func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(ctx)
		},
		migrate.ReadyCheck(db, migrations.FS),
	}
	if svcCtx.CoreControl != nil {
		readyzChecks = append(readyzChecks, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := svcCtx.CoreControl.Health(ctx)
			return err
		})
	}
	if svcCtx.Agent != nil {
		readyzChecks = append(readyzChecks, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := svcCtx.Agent.Health(ctx, &agentv1.AgentHealthRequest{})
			return err
		})
	}

	server.AddRoutes([]rest.Route{
		{Method: "GET", Path: "/healthz", Handler: health.Healthz()},
		{Method: "GET", Path: "/readyz", Handler: health.Readyz(readyzChecks...)},
	})

	handler.RegisterHandlers(server, svcCtx)

	startSessionCleanup(svcCtx)
	startAuditCleanup(svcCtx)

	fmt.Printf("starting %s on %s:%d\n", c.Name, c.Host, c.Port)
	server.Start()
}

// runMigrate 是受控迁移入口：panel migrate <status|up|down> [-f config]。
func runMigrate(args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	configFile := fs.String("f", "etc/panel-api.yaml", "the config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: panel migrate <status|up|down> [-f config]")
		return 2
	}

	c, err := config.Load(*configFile)
	if err != nil {
		log.Printf("load config: %v", err)
		return 1
	}
	if err := c.Validate(); err != nil {
		log.Printf("invalid config: %v", err)
		return 1
	}

	db, err := openDatabase(c)
	if err != nil {
		log.Printf("connect database: %v", err)
		return 1
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	switch fs.Arg(0) {
	case "status":
		statuses, err := migrate.Status(ctx, db, migrations.FS)
		if err != nil {
			log.Printf("migrate status: %v", err)
			return 1
		}
		for _, status := range statuses {
			path := ""
			if status.Source != nil {
				path = status.Source.Path
			}
			fmt.Printf("%-8s %s\n", status.State, path)
		}
	case "up":
		if err := migrate.Up(ctx, db, migrations.FS); err != nil {
			log.Printf("migrate up: %v", err)
			return 1
		}
	case "down":
		if err := migrate.Down(ctx, db, migrations.FS); err != nil {
			log.Printf("migrate down: %v", err)
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown migrate command %q\n", fs.Arg(0))
		return 2
	}
	return 0
}

func openDatabase(c config.Config) (*sql.DB, error) {
	dsn := os.Getenv("RELAYX_DB_DSN")
	if dsn == "" {
		dsn = c.Database.DataSource
	}
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(c.Database.MaxOpenConns)
	db.SetMaxIdleConns(c.Database.MaxIdleConns)
	db.SetConnMaxLifetime(c.Database.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// startSessionCleanup 每小时清理超过保留期的过期会话；进程优雅停机时停止。
func startSessionCleanup(svcCtx *svc.ServiceContext) {
	stop := make(chan struct{})
	var once sync.Once
	shutdown := func() { once.Do(func() { close(stop) }) }
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := svcCtx.SessionManager.Cleanup(context.Background(), nil, sessionRetention)
				if err != nil {
					logx.Errorw("session cleanup failed", logx.Field("error", err.Error()))
					continue
				}
				if n > 0 {
					logx.Infow("session cleanup", logx.Field("deleted", n))
				}
			case <-stop:
				return
			}
		}
	}()
	proc.AddShutdownListener(shutdown)
}

// startAuditCleanup 每日 03:00 UTC 删除超过 180 天的修改密码审计（design.md §4.3）；
// 删除前记录 cutoff，便于审计追溯。进程优雅停机时停止。
func startAuditCleanup(svcCtx *svc.ServiceContext) {
	stop := make(chan struct{})
	var once sync.Once
	shutdown := func() { once.Do(func() { close(stop) }) }
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			timer := time.NewTimer(time.Until(next))
			select {
			case <-timer.C:
				cutoff := time.Now().UTC().Add(-auditRetention)
				logx.Infow("auth.password.audit_cleanup", logx.Field("cutoff", cutoff.Format(time.RFC3339)))
				n, err := svcCtx.AuditModel.DeleteBefore(context.Background(), nil, cutoff)
				if err != nil {
					logx.Errorw("auth.password.audit_cleanup", logx.Field("result", "failure"),
						logx.Field("error", err.Error()))
					continue
				}
				logx.Infow("auth.password.audit_cleanup", logx.Field("result", "success"),
					logx.Field("deleted", n))
			case <-stop:
				timer.Stop()
				return
			}
		}
	}()
	proc.AddShutdownListener(shutdown)
}
