package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ErrNotFound 由 goctl 生成于 vars.go（= sqlx.ErrNotFound / sql.ErrNoRows）。

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
// 写方法接收 sqlx.Session：nil 表示使用连接池（非事务），非 nil 时加入调用方事务。
type AuthUserModel interface {
	Create(ctx context.Context, session sqlx.Session, account, passwordHash, displayName string) (*User, error)
	FindByAccount(ctx context.Context, account string) (*User, error)
	FindOne(ctx context.Context, id string) (*User, error)
	Count(ctx context.Context, session sqlx.Session) (int64, error)
}

// AuthSessionModel 定义会话数据访问。
type AuthSessionModel interface {
	Create(ctx context.Context, session sqlx.Session, s *Session) error
	FindByHash(ctx context.Context, hash []byte) (*SessionWithUser, error)
	Touch(ctx context.Context, session sqlx.Session, id string, lastSeen, expiresAt time.Time) error
	RevokeByHash(ctx context.Context, session sqlx.Session, hash []byte) error
	DeleteExpiredBefore(ctx context.Context, session sqlx.Session, before time.Time) (int64, error)
}
