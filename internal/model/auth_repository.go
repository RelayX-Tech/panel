package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 本文件是生成 model 与领域类型的适配层（design.md §5.1.4）：
// - logic 只依赖 AuthUserModel / AuthSessionModel 接口与领域类型；
// - 主键点查走生成方法（sqlcache）；
// - 写方法接收 sqlx.Session，nil 时使用连接池，非 nil 时加入调用方事务。

type (
	// PostgresAuthUserModel 是 AuthUserModel 的 PostgreSQL 实现。
	PostgresAuthUserModel struct {
		conn      sqlx.SqlConn
		generated AuthUsersModel
	}

	// PostgresAuthSessionModel 是 AuthSessionModel 的 PostgreSQL 实现。
	// 会话查询不缓存；用户点查复用 AuthUserModel.FindOne（sqlcache）。
	PostgresAuthSessionModel struct {
		conn      sqlx.SqlConn
		generated AuthSessionsModel
		users     AuthUserModel
	}
)

var (
	_ AuthUserModel    = (*PostgresAuthUserModel)(nil)
	_ AuthSessionModel = (*PostgresAuthSessionModel)(nil)
)

// NewAuthUserModel 构造用户适配层；生成 model 负责 auth_users 主键点查缓存。
func NewAuthUserModel(conn sqlx.SqlConn, cacheConf cache.CacheConf) *PostgresAuthUserModel {
	return &PostgresAuthUserModel{
		conn:      conn,
		generated: NewAuthUsersModel(conn, cacheConf),
	}
}

// NewAuthSessionModel 构造会话适配层；users 用于按会话取用户（命中 sqlcache）。
func NewAuthSessionModel(conn sqlx.SqlConn, cacheConf cache.CacheConf, users AuthUserModel) *PostgresAuthSessionModel {
	return &PostgresAuthSessionModel{
		conn:      conn,
		generated: NewAuthSessionsModel(conn, cacheConf),
		users:     users,
	}
}

// Create 写入用户；主键由应用生成，避免依赖 RETURNING。
func (m *PostgresAuthUserModel) Create(ctx context.Context, session sqlx.Session, account, passwordHash, displayName string) (*User, error) {
	id := newID()

	// 无事务：走生成 Insert（缓存失效由 sqlcache 维护），再回读默认时间戳。
	if session == nil {
		data := &AuthUsers{Id: id, Account: account, PasswordHash: passwordHash, DisplayName: displayName}
		if _, err := m.generated.Insert(ctx, data); err != nil {
			return nil, fmt.Errorf("create auth user: %w", err)
		}
		got, err := m.generated.FindOne(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("reload auth user: %w", err)
		}
		return toDomainUser(got), nil
	}

	// 事务内：参数化写入（新主键无缓存条目），随后在同一会话回读。
	const insertSQL = `INSERT INTO auth_users (id, account, password_hash, display_name) VALUES ($1, $2, $3, $4)`
	if _, err := execAffected(ctx, m.conn, session, insertSQL, id, account, passwordHash, displayName); err != nil {
		return nil, fmt.Errorf("create auth user: %w", err)
	}
	const selectSQL = `SELECT id, account, password_hash, display_name, disabled_at, created_at, updated_at
FROM auth_users WHERE id = $1`
	row := &AuthUsers{}
	if err := queryRowCtx(ctx, m.conn, session, row, selectSQL, id); err != nil {
		return nil, fmt.Errorf("reload auth user: %w", err)
	}
	return toDomainUser(row), nil
}

// FindOne 按主键点查，走生成方法并命中 sqlcache。
func (m *PostgresAuthUserModel) FindOne(ctx context.Context, id string) (*User, error) {
	row, err := m.generated.FindOne(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find auth user: %w", err)
	}
	return toDomainUser(row), nil
}

// newID 在应用内生成 UUID v4（design.md §5.1.5：不依赖 RETURNING）。
func newID() string {
	return uuid.NewString()
}

func toDomainUser(u *AuthUsers) *User {
	user := &User{
		ID:           u.Id,
		Account:      u.Account,
		PasswordHash: u.PasswordHash,
		DisplayName:  u.DisplayName,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
	if u.DisabledAt.Valid {
		disabledAt := u.DisabledAt.Time
		user.DisabledAt = &disabledAt
	}
	return user
}

func toDomainSession(s *AuthSessions) Session {
	session := Session{
		ID:                s.Id,
		SessionHash:       []byte(s.SessionHash),
		UserID:            s.UserId,
		CreatedAt:         s.CreatedAt,
		LastSeenAt:        s.LastSeenAt,
		ExpiresAt:         s.ExpiresAt,
		AbsoluteExpiresAt: s.AbsoluteExpiresAt,
		ClientIP:          s.ClientIp.String,
		UserAgent:         s.UserAgent.String,
	}
	if s.RevokedAt.Valid {
		revokedAt := s.RevokedAt.Time
		session.RevokedAt = &revokedAt
	}
	return session
}

// execAffected 在连接池或调用方事务上执行写语句，返回影响行数。
func execAffected(ctx context.Context, conn sqlx.SqlConn, session sqlx.Session, query string, args ...any) (int64, error) {
	if session != nil {
		res, err := session.ExecCtx(ctx, query, args...)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	res, err := conn.ExecCtx(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// queryRowCtx 在连接池或调用方事务上执行单行查询。
func queryRowCtx(ctx context.Context, conn sqlx.SqlConn, session sqlx.Session, v any, query string, args ...any) error {
	if session != nil {
		return session.QueryRowCtx(ctx, v, query, args...)
	}
	return conn.QueryRowCtx(ctx, v, query, args...)
}
