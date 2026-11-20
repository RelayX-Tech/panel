package testsupport

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// DefaultTestDSN 指向本地测试容器（docker compose / 开发机）。
const DefaultTestDSN = "postgres://relayx:relayx@127.0.0.1:55432/relayx_test?sslmode=disable"

// PostgresDSN 返回集成测试使用的 DSN，可用 RELAYX_TEST_DSN 覆盖。
func PostgresDSN() string {
	if v := os.Getenv("RELAYX_TEST_DSN"); v != "" {
		return v
	}
	return DefaultTestDSN
}

// NewPostgres 打开测试库连接，并为本次测试创建独立 schema（search_path 隔离），
// 测试结束后删除 schema。连接失败时跳过测试。
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

	schema := fmt.Sprintf("test_%s", randomSuffix(t))
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+quoteIdent(schema)); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + quoteIdent(schema) + " CASCADE")
		_ = admin.Close()
	})

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := sql.Open("pgx", dsn+sep+"search_path="+schema)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	db.SetMaxOpenConns(4)
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	return db
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// randomSuffix 使用 OS 随机数生成 schema 名后缀，避免并行测试进程间的命名碰撞。
func randomSuffix(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("random schema suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}
