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

// PasswordAudit 是 auth_password_audits 的领域表示；不含任何密码 / 哈希 / 凭据。
type PasswordAudit struct {
	ID         string
	UserID     string
	RequestID  string
	ClientIP   string
	Result     string
	OccurredAt time.Time
}

// AuthUserModel 定义账号数据访问；logic 依赖该接口以便单测注入假实现。
// 写方法接收 sqlx.Session：nil 表示使用连接池（非事务），非 nil 时加入调用方事务。
type AuthUserModel interface {
	Create(ctx context.Context, session sqlx.Session, account, passwordHash, displayName string) (*User, error)
	FindByAccount(ctx context.Context, account string) (*User, error)
	FindOne(ctx context.Context, id string) (*User, error)
	LockByID(ctx context.Context, session sqlx.Session, id string) (*User, error)
	UpdatePasswordHash(ctx context.Context, session sqlx.Session, id, passwordHash string) error
	InvalidateCache(ctx context.Context, id string) error
	Count(ctx context.Context, session sqlx.Session) (int64, error)
}

// AuthSessionModel 定义会话数据访问。
type AuthSessionModel interface {
	Create(ctx context.Context, session sqlx.Session, s *Session) error
	FindByHash(ctx context.Context, hash []byte) (*SessionWithUser, error)
	Touch(ctx context.Context, session sqlx.Session, id string, lastSeen, expiresAt time.Time) error
	RevokeByHash(ctx context.Context, session sqlx.Session, hash []byte) error
	RevokeAllByUserID(ctx context.Context, session sqlx.Session, userID string) (int64, error)
	DeleteExpiredBefore(ctx context.Context, session sqlx.Session, before time.Time) (int64, error)
}

// AuthPasswordAuditModel 定义修改密码审计数据访问（只追加；按 user_id / request_id 检索）。
type AuthPasswordAuditModel interface {
	Insert(ctx context.Context, session sqlx.Session, audit *PasswordAudit) error
	ListByUserID(ctx context.Context, userID string, limit int) ([]PasswordAudit, error)
	FindByRequestID(ctx context.Context, requestID string) ([]PasswordAudit, error)
	DeleteBefore(ctx context.Context, session sqlx.Session, before time.Time) (int64, error)
}
