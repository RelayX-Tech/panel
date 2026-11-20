package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound 表示查询目标不存在。
var ErrNotFound = errors.New("not found")

// User 是 auth_users 的领域表示。
type User struct {
	ID           string
	Account      string
	PasswordHash string
	DisplayName  string
	DisabledAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session 是 auth_sessions 的领域表示。
type Session struct {
	ID                string
	SessionHash       []byte
	UserID            string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	ClientIP          string
	UserAgent         string
}

// SessionWithUser 是会话查询的联表结果。
type SessionWithUser struct {
	Session Session
	User    User
}

// AuthUserModel 定义账号数据访问；logic 依赖该接口以便单测注入假实现。
type AuthUserModel interface {
	Create(ctx context.Context, account, passwordHash, displayName string) (*User, error)
	FindByAccount(ctx context.Context, account string) (*User, error)
	Count(ctx context.Context) (int64, error)
}

// AuthSessionModel 定义会话数据访问。
type AuthSessionModel interface {
	Create(ctx context.Context, s *Session) error
	FindByHash(ctx context.Context, hash []byte) (*SessionWithUser, error)
	Touch(ctx context.Context, id string, lastSeen, expiresAt time.Time) error
	RevokeByHash(ctx context.Context, hash []byte) error
	DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error)
}

// PostgresAuthUserModel 是 AuthUserModel 的 PostgreSQL 实现。
type PostgresAuthUserModel struct {
	db *sql.DB
}

func NewAuthUserModel(db *sql.DB) *PostgresAuthUserModel {
	return &PostgresAuthUserModel{db: db}
}

func (m *PostgresAuthUserModel) Create(ctx context.Context, account, passwordHash, displayName string) (*User, error) {
	const q = `
INSERT INTO auth_users (account, password_hash, display_name)
VALUES ($1, $2, $3)
RETURNING id::text, account, password_hash, display_name, disabled_at, created_at, updated_at`
	u := &User{}
	err := m.db.QueryRowContext(ctx, q, account, passwordHash, displayName).
		Scan(&u.ID, &u.Account, &u.PasswordHash, &u.DisplayName, &u.DisabledAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create auth user: %w", err)
	}
	return u, nil
}

func (m *PostgresAuthUserModel) FindByAccount(ctx context.Context, account string) (*User, error) {
	const q = `
SELECT id::text, account, password_hash, display_name, disabled_at, created_at, updated_at
FROM auth_users
WHERE lower(account) = lower($1)`
	u := &User{}
	err := m.db.QueryRowContext(ctx, q, account).
		Scan(&u.ID, &u.Account, &u.PasswordHash, &u.DisplayName, &u.DisabledAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find auth user: %w", err)
	}
	return u, nil
}

func (m *PostgresAuthUserModel) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := m.db.QueryRowContext(ctx, `SELECT count(*) FROM auth_users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count auth users: %w", err)
	}
	return n, nil
}

// PostgresAuthSessionModel 是 AuthSessionModel 的 PostgreSQL 实现。
type PostgresAuthSessionModel struct {
	db *sql.DB
}

func NewAuthSessionModel(db *sql.DB) *PostgresAuthSessionModel {
	return &PostgresAuthSessionModel{db: db}
}

func (m *PostgresAuthSessionModel) Create(ctx context.Context, s *Session) error {
	const q = `
INSERT INTO auth_sessions
    (session_hash, user_id, created_at, last_seen_at, expires_at, absolute_expires_at, client_ip, user_agent)
VALUES ($1, $2::uuid, $3, $4, $5, $6, nullif($7, '')::inet, nullif($8, ''))
RETURNING id::text`
	err := m.db.QueryRowContext(ctx, q,
		s.SessionHash, s.UserID, s.CreatedAt, s.LastSeenAt, s.ExpiresAt, s.AbsoluteExpiresAt,
		s.ClientIP, s.UserAgent,
	).Scan(&s.ID)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (m *PostgresAuthSessionModel) FindByHash(ctx context.Context, hash []byte) (*SessionWithUser, error) {
	const q = `
SELECT s.id::text, s.session_hash, s.user_id::text, s.created_at, s.last_seen_at,
       s.expires_at, s.absolute_expires_at, s.revoked_at,
       coalesce(host(s.client_ip), ''), coalesce(s.user_agent, ''),
       u.id::text, u.account, u.password_hash, u.display_name, u.disabled_at, u.created_at, u.updated_at
FROM auth_sessions s
JOIN auth_users u ON u.id = s.user_id
WHERE s.session_hash = $1`
	out := &SessionWithUser{}
	err := m.db.QueryRowContext(ctx, q, hash).Scan(
		&out.Session.ID, &out.Session.SessionHash, &out.Session.UserID,
		&out.Session.CreatedAt, &out.Session.LastSeenAt, &out.Session.ExpiresAt,
		&out.Session.AbsoluteExpiresAt, &out.Session.RevokedAt,
		&out.Session.ClientIP, &out.Session.UserAgent,
		&out.User.ID, &out.User.Account, &out.User.PasswordHash, &out.User.DisplayName,
		&out.User.DisabledAt, &out.User.CreatedAt, &out.User.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find session: %w", err)
	}
	return out, nil
}

func (m *PostgresAuthSessionModel) Touch(ctx context.Context, id string, lastSeen, expiresAt time.Time) error {
	const q = `UPDATE auth_sessions SET last_seen_at = $2, expires_at = $3 WHERE id = $1::uuid`
	if _, err := m.db.ExecContext(ctx, q, id, lastSeen, expiresAt); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func (m *PostgresAuthSessionModel) RevokeByHash(ctx context.Context, hash []byte) error {
	const q = `UPDATE auth_sessions SET revoked_at = now() WHERE session_hash = $1 AND revoked_at IS NULL`
	if _, err := m.db.ExecContext(ctx, q, hash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (m *PostgresAuthSessionModel) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	const q = `DELETE FROM auth_sessions WHERE expires_at < $1`
	res, err := m.db.ExecContext(ctx, q, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}
