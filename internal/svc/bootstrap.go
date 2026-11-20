package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/model"
)

// adminBootstrapLockKey 串行化多实例并发的「检查空表 → 创建管理员」。
const adminBootstrapLockKey int64 = 0x52454C41444D // "RELADM"

// MinAdminPasswordLength 是初始管理员密码的最小长度。
const MinAdminPasswordLength = 12

// AdminBootstrapResult 描述引导结果；Warning 非空时由调用方写入启动日志。
type AdminBootstrapResult struct {
	Created bool
	Account string
	Warning string
}

// BootstrapAdminFromEnv 读取 RELAYX_ADMIN_ACCOUNT / RELAYX_ADMIN_PASSWORD 引导初始管理员。
func BootstrapAdminFromEnv(ctx context.Context, db *sql.DB, users model.AuthUserModel) (AdminBootstrapResult, error) {
	account := os.Getenv("RELAYX_ADMIN_ACCOUNT")
	password := os.Getenv("RELAYX_ADMIN_PASSWORD")
	if account == "" || password == "" {
		return AdminBootstrapResult{
			Warning: "RELAYX_ADMIN_ACCOUNT/RELAYX_ADMIN_PASSWORD not set; admin bootstrap skipped",
		}, nil
	}
	return BootstrapAdmin(ctx, db, users, account, password)
}

// BootstrapAdmin 在 auth_users 为空时创建初始管理员；已有用户则跳过。
// 密码只经环境变量/secret 注入，不写日志；结果只含账号。
func BootstrapAdmin(ctx context.Context, db *sql.DB, users model.AuthUserModel, account, password string) (AdminBootstrapResult, error) {
	if len(password) < MinAdminPasswordLength {
		return AdminBootstrapResult{}, fmt.Errorf("admin password must be at least %d characters", MinAdminPasswordLength)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return AdminBootstrapResult{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, adminBootstrapLockKey); err != nil {
		return AdminBootstrapResult{}, fmt.Errorf("acquire bootstrap lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, adminBootstrapLockKey)
	}()

	var n int64
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM auth_users`).Scan(&n); err != nil {
		return AdminBootstrapResult{}, fmt.Errorf("count auth users: %w", err)
	}
	if n > 0 {
		return AdminBootstrapResult{}, nil
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return AdminBootstrapResult{}, fmt.Errorf("hash admin password: %w", err)
	}
	if _, err := users.Create(ctx, account, hash, "Administrator"); err != nil {
		return AdminBootstrapResult{}, fmt.Errorf("create admin: %w", err)
	}
	return AdminBootstrapResult{Created: true, Account: account}, nil
}

// ErrNoAdminConfigured 用于需要显式提示「未配置初始管理员」的场景。
var ErrNoAdminConfigured = errors.New("no admin configured")
