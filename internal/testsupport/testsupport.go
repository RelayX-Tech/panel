package testsupport

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DefaultTestDSN 指向本地测试容器（docker compose / 开发机）。
const DefaultTestDSN = "postgres://relayx:relayx@127.0.0.1:55432/relayx_test?sslmode=disable"

// DefaultRedisAddr 与 compose.dev.yml 的 redis 端口对齐。
const DefaultRedisAddr = "127.0.0.1:56379"

// PostgresDSN 返回集成测试使用的 DSN，可用 RELAYX_TEST_DSN 覆盖。
func PostgresDSN() string {
	if v := os.Getenv("RELAYX_TEST_DSN"); v != "" {
		return v
	}
	return DefaultTestDSN
}

// RedisAddr 返回 sqlcache 使用的 Redis 地址，可用 REDIS_ADDR 覆盖。
func RedisAddr() string {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		return v
	}
	return DefaultRedisAddr
}

// CacheConf 返回指向测试 Redis 的 sqlcache 配置。
func CacheConf() cache.CacheConf {
	return cache.CacheConf{{RedisConf: redis.RedisConf{Host: RedisAddr(), Type: "node"}, Weight: 100}}
}

// Conn 把测试 *sql.DB 包装为 go-zero 业务连接（复用同一连接池）。
func Conn(db *sql.DB) sqlx.SqlConn {
	return sqlx.NewSqlConnFromDB(db)
}

// RequireRedis 确认测试 Redis 可达；不可达时失败并给出启动命令。
func RequireRedis(t *testing.T) {
	t.Helper()
	rds := (redis.RedisConf{Host: RedisAddr(), Type: "node"}).NewRedis()
	if !rds.Ping() {
		t.Fatalf("redis unavailable at %s; run: docker compose -f compose.dev.yml up -d redis", RedisAddr())
	}
}

// NewPostgres 打开测试库连接。
//
// 每个用例创建独立数据库（而非独立 schema）：goctl 生成的 model 会把表名限定为
// `"public"."<table>"`，search_path 隔离不生效，故用独立库保持用例隔离。
// 连接失败时跳过测试；创建数据库失败时同样跳过（需要 CREATEDB 权限）。
func NewPostgres(t *testing.T) *sql.DB {
	t.Helper()

	dsn := PostgresDSN()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Skipf("postgres unavailable: %v", err)
	}

	dbName := "test_" + randomSuffix(t)
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+quoteIdent(dbName)); err != nil {
		_ = admin.Close()
		t.Skipf("create test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + quoteIdent(dbName) + " WITH (FORCE)")
		_ = admin.Close()
	})

	db, err := sql.Open("pgx", dsnWithDatabase(dsn, dbName))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// t.Cleanup 后进先出：先关闭连接池，再删除数据库。
	t.Cleanup(func() { _ = db.Close() })

	db.SetMaxOpenConns(4)
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	return db
}

func dsnWithDatabase(dsn, name string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return dsn
	}
	u.Path = "/" + name
	return u.String()
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// randomSuffix 使用 OS 随机数生成数据库名后缀，避免并行测试进程间的命名碰撞。
func randomSuffix(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}
