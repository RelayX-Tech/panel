package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 本文件是手写扩展查询（design.md §5.1.5）：条件查询、联表、批量写入。
// 约束：全部使用 $n 占位符；条件 / 联表查询不走 sqlcache；写方法接收 sqlx.Session。

// FindByAccount 按账号大小写不敏感查询；不缓存。
func (m *PostgresAuthUserModel) FindByAccount(ctx context.Context, account string) (*User, error) {
	const query = `SELECT id, account, password_hash, display_name, disabled_at, created_at, updated_at
FROM auth_users
WHERE lower(account) = lower($1)`
	row := &AuthUsers{}
	if err := m.conn.QueryRowCtx(ctx, row, query, account); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find auth user: %w", err)
	}
	return toDomainUser(row), nil
}

// Count 返回用户总数；可在事务内调用。
func (m *PostgresAuthUserModel) Count(ctx context.Context, session sqlx.Session) (int64, error) {
	const query = `SELECT count(*) FROM auth_users`
	var n int64
	if err := queryRowCtx(ctx, m.conn, session, &n, query); err != nil {
		return 0, fmt.Errorf("count auth users: %w", err)
	}
	return n, nil
}

// Create 写入会话；主键由应用生成，client_ip 按 inet 解析。
// revoked_at 建会话时恒为 NULL：go-zero sqlx 的语句日志无法格式化 nil *time.Time，
// 故不把该列作为参数传入，交由表默认值（NULL）处理。
func (m *PostgresAuthSessionModel) Create(ctx context.Context, session sqlx.Session, s *Session) error {
	s.ID = newID()
	const query = `INSERT INTO auth_sessions
    (id, session_hash, user_id, created_at, last_seen_at, expires_at, absolute_expires_at, client_ip, user_agent)
VALUES ($1, $2, $3::uuid, $4, $5, $6, $7, nullif($8, '')::inet, nullif($9, ''))`
	if _, err := execAffected(ctx, m.conn, session, query,
		s.ID, s.SessionHash, s.UserID, s.CreatedAt, s.LastSeenAt, s.ExpiresAt, s.AbsoluteExpiresAt,
		s.ClientIP, s.UserAgent,
	); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// FindByHash 按 session_hash 查会话，再用缓存的用户主键点查组装结果；会话本身不缓存。
func (m *PostgresAuthSessionModel) FindByHash(ctx context.Context, hash []byte) (*SessionWithUser, error) {
	const query = `SELECT id::text AS id, session_hash, user_id::text AS user_id, created_at, last_seen_at,
       expires_at, absolute_expires_at, revoked_at,
       coalesce(host(client_ip), '') AS client_ip, coalesce(user_agent, '') AS user_agent
FROM auth_sessions
WHERE session_hash = $1`
	row := &AuthSessions{}
	if err := m.conn.QueryRowCtx(ctx, row, query, hash); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find session: %w", err)
	}

	user, err := m.users.FindOne(ctx, row.UserId)
	if errors.Is(err, ErrNotFound) {
		// 外键保证正常情况下不会发生（design.md §5.1.5）。
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &SessionWithUser{Session: toDomainSession(row), User: *user}, nil
}

// Touch 滑动续期；可在事务内调用。
func (m *PostgresAuthSessionModel) Touch(ctx context.Context, session sqlx.Session, id string, lastSeen, expiresAt time.Time) error {
	const query = `UPDATE auth_sessions SET last_seen_at = $2, expires_at = $3 WHERE id = $1::uuid`
	if _, err := execAffected(ctx, m.conn, session, query, id, lastSeen, expiresAt); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// RevokeByHash 吊销会话；幂等（已吊销时不再更新）。可在事务内调用。
func (m *PostgresAuthSessionModel) RevokeByHash(ctx context.Context, session sqlx.Session, hash []byte) error {
	const query = `UPDATE auth_sessions SET revoked_at = now() WHERE session_hash = $1 AND revoked_at IS NULL`
	if _, err := execAffected(ctx, m.conn, session, query, hash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// RevokeAllByUserID 吊销该用户全部未吊销会话（含当前设备）；返回影响行数。
// 修改密码成功后调用，其他设备下一次请求由 Auth 中间件返回 AUTH_SESSION_EXPIRED。
func (m *PostgresAuthSessionModel) RevokeAllByUserID(ctx context.Context, session sqlx.Session, userID string) (int64, error) {
	const query = `UPDATE auth_sessions SET revoked_at = now() WHERE user_id = $1::uuid AND revoked_at IS NULL`
	n, err := execAffected(ctx, m.conn, session, query, userID)
	if err != nil {
		return 0, fmt.Errorf("revoke sessions by user: %w", err)
	}
	return n, nil
}

// DeleteExpiredBefore 删除过期超过保留期的会话；可在事务内调用。
func (m *PostgresAuthSessionModel) DeleteExpiredBefore(ctx context.Context, session sqlx.Session, before time.Time) (int64, error) {
	const query = `DELETE FROM auth_sessions WHERE expires_at < $1`
	n, err := execAffected(ctx, m.conn, session, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return n, nil
}
