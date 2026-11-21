package middleware

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/relayx-tech/panel/internal/response"
)

// OriginCheckMiddleware 对写操作校验 Origin 精确白名单与 Sec-Fetch-Site（CSRF 防线）。
type OriginCheckMiddleware struct {
	allowed map[string]struct{}
}

func NewOriginCheckMiddleware(allowedOrigins []string) *OriginCheckMiddleware {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return &OriginCheckMiddleware{allowed: allowed}
}

func (m *OriginCheckMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next(w, r)
			return
		}

		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			httpx.ErrorCtx(r.Context(), w, response.CSRFRejected())
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			httpx.ErrorCtx(r.Context(), w, response.CSRFRejected())
			return
		}
		if _, ok := m.allowed[origin]; !ok {
			httpx.ErrorCtx(r.Context(), w, response.CSRFRejected())
			return
		}
		next(w, r)
	}
}
