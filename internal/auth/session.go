package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// sessionTokenBytes 是会话 token 的随机字节数（32 字节 → base64url 43 字符）。
const sessionTokenBytes = 32

// GenerateSessionToken 生成 256-bit 随机 token（base64url，无填充）。
func GenerateSessionToken() (string, error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashSessionToken 返回 token 的 sha256（数据库只存该哈希）。
func HashSessionToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// NewSessionExpiry 计算空闲到期与绝对到期：
// 空闲到期 = min(now + idleTTL, created + absoluteTTL)，绝对到期 = created + absoluteTTL。
func NewSessionExpiry(now, created time.Time, idleTTL, absoluteTTL time.Duration) (time.Time, time.Time) {
	absolute := created.Add(absoluteTTL)
	idle := now.Add(idleTTL)
	if idle.After(absolute) {
		idle = absolute
	}
	return idle, absolute
}

// ShouldTouch 判断距上次活跃是否已达 touch 门槛（避免写放大）。
func ShouldTouch(lastSeen, now time.Time, interval time.Duration) bool {
	return !now.Before(lastSeen.Add(interval))
}

// Validity 是会话有效性判定结果。
type Validity int

const (
	SessionValid Validity = iota
	SessionExpiredState
)

// SessionState 是判定会话有效性所需的快照。
type SessionState struct {
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         time.Time
	UserDisabled      bool
}

// Evaluate 判定会话是否有效：过期、吊销、超过绝对上限、账号禁用均视为失效。
func (s SessionState) Evaluate(now time.Time) Validity {
	if !s.RevokedAt.IsZero() || s.UserDisabled {
		return SessionExpiredState
	}
	if !now.Before(s.ExpiresAt) || !now.Before(s.AbsoluteExpiresAt) {
		return SessionExpiredState
	}
	return SessionValid
}
