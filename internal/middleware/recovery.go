package middleware

import (
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"

	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
)

// Recovery 捕获 panic，返回统一 500 包装，不暴露堆栈。
func Recovery() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logx.WithContext(r.Context()).Errorw("panic recovered",
						logx.Field("requestId", requestid.FromContext(r.Context())),
						logx.Field("method", r.Method),
						logx.Field("path", r.URL.Path))
					httpx.ErrorCtx(r.Context(), w, response.Internal(nil))
				}
			}()
			next(w, r)
		}
	}
}

// AccessLog 记录方法、路径、状态、耗时与 requestId；不记录请求体与凭据。
func AccessLog() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next(rec, r)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			logx.WithContext(r.Context()).Infow("access",
				logx.Field("requestId", requestid.FromContext(r.Context())),
				logx.Field("method", r.Method),
				logx.Field("route", r.URL.Path),
				logx.Field("status", status),
				logx.Field("latencyMs", time.Since(start).Milliseconds()),
				logx.Field("clientIp", RemoteIP(r, false, nil)),
			)
		}
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Flush 保证流式响应（如 SSE）不被包装阻断。
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
