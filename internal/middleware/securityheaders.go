package middleware

import "net/http"

// contentSecurityPolicy 是 Panel 管理面的默认 CSP。
// next-themes 的首屏内联脚本需要 'unsafe-inline'；生产可由部署层替换为 nonce/hash。
const contentSecurityPolicy = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// SecurityHeaders 统一设置安全响应头（design.md §3.2、§4.6）。
func SecurityHeaders() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Content-Security-Policy", contentSecurityPolicy)
			next(w, r)
		}
	}
}
