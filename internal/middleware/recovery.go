package middleware

import (
	"bufio"
	"net"
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

// Unwrap 暴露底层 writer，使 http.ResponseController 能穿透包装
// 查找 Hijacker / Flusher 等可选能力（go1.20+）。
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

// Hijack 透传连接接管，供 WebSocket 升级使用（C18）。
// gorilla/websocket 经 http.NewResponseController(w).Hijack() 调用；若包装
// 不透传，控制器会返回 http.ErrNotSupported（"feature not supported"）。
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := s.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
