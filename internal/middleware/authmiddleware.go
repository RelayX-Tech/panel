package middleware

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
)

// SessionAuthenticator 是 Auth 中间件依赖的会话校验能力（便于单测注入）。
type SessionAuthenticator interface {
	Authenticate(ctx context.Context, token string) (*model.SessionWithUser, error)
}

type AuthMiddleware struct {
	authenticator SessionAuthenticator
	cookieName    string
}

func NewAuthMiddleware(authenticator SessionAuthenticator, cookieName string) *AuthMiddleware {
	return &AuthMiddleware{authenticator: authenticator, cookieName: cookieName}
}

// Handle 校验会话 Cookie，成功后将会话注入 context 并放行。
func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := response.ReadSessionCookie(r, m.cookieName)
		row, err := m.authenticator.Authenticate(r.Context(), token)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrSessionNotFound):
				httpx.ErrorCtx(r.Context(), w, response.Unauthorized())
			case errors.Is(err, auth.ErrSessionExpired):
				httpx.ErrorCtx(r.Context(), w, response.SessionExpired())
			default:
				httpx.ErrorCtx(r.Context(), w, response.Internal(err))
			}
			return
		}
		next(w, r.WithContext(auth.ContextWithSession(r.Context(), row)))
	}
}

// RemoteIP 返回来源 IP；仅当来源地址命中 TrustedProxyCIDRs 时才采信 X-Forwarded-For，
// 防止伪造头绕过限流。
func RemoteIP(r *http.Request, trustProxy bool, trustedCIDRs []string) string {
	remote := hostPart(r.RemoteAddr)
	if !trustProxy || !inCIDRs(remote, trustedCIDRs) {
		return remote
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, ok := cutFirst(xff); ok {
			return ip
		}
	}
	return remote
}

func hostPart(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func inCIDRs(ip string, cidrs []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, c := range cidrs {
		_, network, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

func cutFirst(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return trimSpace(s[:i]), s[i+1:], true
		}
	}
	return trimSpace(s), "", true
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
