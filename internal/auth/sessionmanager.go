package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/relayx-tech/panel/internal/model"
)

// ErrSessionNotFound 表示请求未携带会话或会话不存在（→ AUTH_UNAUTHORIZED）。
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionExpired 表示会话过期、被吊销或账号被禁用（→ AUTH_SESSION_EXPIRED）。
var ErrSessionExpired = errors.New("session expired")

// SessionManager 负责会话的签发、校验、滑动续期与吊销。
type SessionManager struct {
	Sessions      model.AuthSessionModel
	IdleTTL       time.Duration
	AbsoluteTTL   time.Duration
	TouchInterval time.Duration
	Now           func() time.Time
}

func NewSessionManager(sessions model.AuthSessionModel, idleTTL, absoluteTTL, touchInterval time.Duration) *SessionManager {
	return &SessionManager{
		Sessions:      sessions,
		IdleTTL:       idleTTL,
		AbsoluteTTL:   absoluteTTL,
		TouchInterval: touchInterval,
		Now:           func() time.Time { return time.Now().UTC() },
	}
}

// Create 生成并持久化新会话；返回的明文 token 仅用于写入 Cookie。
// session 为 nil 时使用连接池，非 nil 时加入调用方事务。
func (m *SessionManager) Create(ctx context.Context, session sqlx.Session, userID, clientIP, userAgent string) (string, *model.Session, error) {
	token, err := GenerateSessionToken()
	if err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	now := m.Now()
	expiresAt, absoluteExpiresAt := NewSessionExpiry(now, now, m.IdleTTL, m.AbsoluteTTL)
	sess := &model.Session{
		SessionHash:       HashSessionToken(token),
		UserID:            userID,
		CreatedAt:         now,
		LastSeenAt:        now,
		ExpiresAt:         expiresAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
		ClientIP:          clientIP,
		UserAgent:         userAgent,
	}
	if err := m.Sessions.Create(ctx, session, sess); err != nil {
		return "", nil, err
	}
	return token, sess, nil
}

// Authenticate 校验会话并按需滑动续期；失败返回 ErrSessionNotFound / ErrSessionExpired。
func (m *SessionManager) Authenticate(ctx context.Context, token string) (*model.SessionWithUser, error) {
	if token == "" {
		return nil, ErrSessionNotFound
	}
	row, err := m.Sessions.FindByHash(ctx, HashSessionToken(token))
	if errors.Is(err, model.ErrNotFound) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	now := m.Now()
	state := SessionState{
		ExpiresAt:         row.Session.ExpiresAt,
		AbsoluteExpiresAt: row.Session.AbsoluteExpiresAt,
		UserDisabled:      row.User.DisabledAt != nil,
	}
	if row.Session.RevokedAt != nil {
		state.RevokedAt = *row.Session.RevokedAt
	}
	if state.Evaluate(now) != SessionValid {
		return nil, ErrSessionExpired
	}

	if ShouldTouch(row.Session.LastSeenAt, now, m.TouchInterval) {
		expiresAt, _ := NewSessionExpiry(now, row.Session.CreatedAt, m.IdleTTL, m.AbsoluteTTL)
		if err := m.Sessions.Touch(ctx, nil, row.Session.ID, now, expiresAt); err != nil {
			return nil, err
		}
		row.Session.LastSeenAt = now
		row.Session.ExpiresAt = expiresAt
	}
	return row, nil
}

// Revoke 吊销会话；token 为空或无对应会话时视为成功（幂等）。
func (m *SessionManager) Revoke(ctx context.Context, session sqlx.Session, token string) error {
	if token == "" {
		return nil
	}
	return m.Sessions.RevokeByHash(ctx, session, HashSessionToken(token))
}

// Cleanup 删除过期超过 retention 的会话行。
func (m *SessionManager) Cleanup(ctx context.Context, session sqlx.Session, retention time.Duration) (int64, error) {
	return m.Sessions.DeleteExpiredBefore(ctx, session, m.Now().Add(-retention))
}

type sessionCtxKey struct{}

// ContextWithSession 把已校验会话注入上下文（供 Session logic 使用）。
func ContextWithSession(ctx context.Context, row *model.SessionWithUser) context.Context {
	return context.WithValue(ctx, sessionCtxKey{}, row)
}

// SessionFromContext 读取上下文中的已校验会话。
func SessionFromContext(ctx context.Context) *model.SessionWithUser {
	row, _ := ctx.Value(sessionCtxKey{}).(*model.SessionWithUser)
	return row
}
