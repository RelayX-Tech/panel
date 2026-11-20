package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
)

type rateBucket struct {
	windowStart time.Time
	count       int
}

// LoginRateLimitMiddleware 是按来源 IP 的固定窗口限流（仅作用于登录路径）。
type LoginRateLimitMiddleware struct {
	Window   time.Duration
	Max      int
	OnlyPath string
	Now      func() time.Time

	// TrustProxy/TrustedProxyCIDRs 控制是否采信 X-Forwarded-For。
	TrustProxy        bool
	TrustedProxyCIDRs []string

	mu      sync.Mutex
	buckets map[string]*rateBucket
}

func NewLoginRateLimitMiddleware(window time.Duration, maxAttempts int, onlyPath string) *LoginRateLimitMiddleware {
	return &LoginRateLimitMiddleware{
		Window:   window,
		Max:      maxAttempts,
		OnlyPath: onlyPath,
		Now:      func() time.Time { return time.Now() },
		buckets:  make(map[string]*rateBucket),
	}
}

func (m *LoginRateLimitMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.OnlyPath != "" && r.URL.Path != m.OnlyPath {
			next(w, r)
			return
		}

		ip := RemoteIP(r, m.TrustProxy, m.TrustedProxyCIDRs)
		now := m.Now()

		m.mu.Lock()
		if len(m.buckets) > 10000 {
			for key, b := range m.buckets {
				if now.Sub(b.windowStart) >= 2*m.Window {
					delete(m.buckets, key)
				}
			}
		}
		b, ok := m.buckets[ip]
		if !ok || now.Sub(b.windowStart) >= m.Window {
			b = &rateBucket{windowStart: now}
			m.buckets[ip] = b
		}
		b.count++
		count := b.count
		remaining := m.Window - now.Sub(b.windowStart)
		m.mu.Unlock()

		if count > m.Max {
			retryAfter := int(remaining.Seconds() + 0.999)
			if retryAfter < 1 {
				retryAfter = 1
			}
			logx.WithContext(r.Context()).Infow("auth.login.rate_limited",
				logx.Field("requestId", requestid.FromContext(r.Context())),
				logx.Field("clientIp", ip),
				logx.Field("result", "rate_limited"))
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			httpx.ErrorCtx(r.Context(), w, response.RateLimited(retryAfter))
			return
		}
		next(w, r)
	}
}
