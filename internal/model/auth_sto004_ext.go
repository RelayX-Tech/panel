package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 本文件是 STO-004 对既有 auth model 的扩展（显示名、会话列表与吊销）。
// 约束与 auth_queries_ext.go 一致：$n 占位符、写方法接收 sqlx.Session、不缓存。
// 扩展能力以窄接口暴露，避免破坏既有 AuthUserModel / AuthSessionModel 的测试替身。

// DisplayNameUpdater 是显示名更新能力。
type DisplayNameUpdater interface {
	UpdateDisplayName(ctx context.Context, session sqlx.Session, id, displayName string) error
}

// SessionAdminModel 是会话管理能力（列表 / 点查 / 按 ID 吊销）。
type SessionAdminModel interface {
	FindByID(ctx context.Context, id string) (*Session, error)
	ListByUserID(ctx context.Context, userID string) ([]Session, error)
	RevokeByID(ctx context.Context, session sqlx.Session, id string) error
}

// RoleModel 是角色读取能力（客户端统计等管理员守卫）。
type RoleModel interface {
	RoleOf(ctx context.Context, userID string) (string, error)
}

var (
	_ DisplayNameUpdater = (*PostgresAuthUserModel)(nil)
	_ SessionAdminModel  = (*PostgresAuthSessionModel)(nil)
	_ RoleModel          = (*PostgresAuthUserModel)(nil)
)

// RoleOf 返回账户角色；无记录时返回空串。
func (m *PostgresAuthUserModel) RoleOf(ctx context.Context, userID string) (string, error) {
	var role string
	const query = `SELECT role FROM auth_users WHERE id = $1::uuid`
	if err := m.conn.QueryRowCtx(ctx, &role, query, userID); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("read role: %w", err)
	}
	return role, nil
}

const sessionSelectColumns = `id::text AS id, session_hash, user_id::text AS user_id,
       created_at, last_seen_at, expires_at, absolute_expires_at, revoked_at,
       coalesce(host(client_ip), '') AS client_ip, coalesce(user_agent, '') AS user_agent
FROM auth_sessions`

func (m *PostgresAuthUserModel) UpdateDisplayName(ctx context.Context, session sqlx.Session, id, displayName string) error {
	const query = `UPDATE auth_users SET display_name = $1, updated_at = now() WHERE id = $2`
	if _, err := execAffected(ctx, m.conn, session, query, displayName, id); err != nil {
		return fmt.Errorf("update display name: %w", err)
	}
	return m.InvalidateCache(ctx, id)
}

func (m *PostgresAuthSessionModel) ListByUserID(ctx context.Context, userID string) ([]Session, error) {
	query := `SELECT ` + sessionSelectColumns + `
WHERE user_id = $1::uuid AND revoked_at IS NULL AND absolute_expires_at > now()
ORDER BY last_seen_at DESC`
	var rows []AuthSessions
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, userID); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	out := make([]Session, 0, len(rows))
	for i := range rows {
		out = append(out, toDomainSession(&rows[i]))
	}
	return out, nil
}

func (m *PostgresAuthSessionModel) FindByID(ctx context.Context, id string) (*Session, error) {
	query := `SELECT ` + sessionSelectColumns + ` WHERE id = $1::uuid`
	var row AuthSessions
	if err := m.conn.QueryRowCtx(ctx, &row, query, id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find session: %w", err)
	}
	s := toDomainSession(&row)
	return &s, nil
}

func (m *PostgresAuthSessionModel) RevokeByID(ctx context.Context, session sqlx.Session, id string) error {
	const query = `UPDATE auth_sessions SET revoked_at = now() WHERE id = $1::uuid AND revoked_at IS NULL`
	if _, err := execAffected(ctx, m.conn, session, query, id); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}
