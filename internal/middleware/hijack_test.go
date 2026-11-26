package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// hijackableWriter 是支持 http.Hijacker 的底层 ResponseWriter。
type hijackableWriter struct {
	header   http.Header
	hijacked bool
}

func (w *hijackableWriter) Header() http.Header         { return w.header }
func (w *hijackableWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *hijackableWriter) WriteHeader(int)             {}

func (w *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	return nil, nil, nil
}

// TestAccessLogPreservesHijackerForWebSocket 覆盖 C18：AccessLog 的 statusRecorder
// 必须透传 http.Hijacker，否则 gorilla（经 http.NewResponseController）升级 WS 失败。
func TestAccessLogPreservesHijackerForWebSocket(t *testing.T) {
	under := &hijackableWriter{header: http.Header{}}
	var directOK, controllerOK, unwrapOK bool

	h := AccessLog()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Hijacker); ok {
			directOK = true
		}
		if u, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok && u.Unwrap() == under {
			unwrapOK = true
		}
		if _, _, err := http.NewResponseController(w).Hijack(); err == nil {
			controllerOK = true
		}
	}))
	h.ServeHTTP(under, httptest.NewRequest(http.MethodGet, "/api/v1/ws", nil))

	if !directOK {
		t.Fatal("wrapped writer must implement http.Hijacker")
	}
	if !unwrapOK {
		t.Fatal("wrapped writer must expose Unwrap() to the underlying writer")
	}
	if !controllerOK {
		t.Fatal("http.NewResponseController(w).Hijack() must succeed through AccessLog")
	}
	if !under.hijacked {
		t.Fatal("Hijack must delegate to the underlying writer")
	}
}

// TestAccessLogHijackUnsupportedSurfaceErrNotSupported 保证底层不支持时返回标准错误，
// 不 panic、不静默成功。
func TestAccessLogHijackUnsupportedSurfacesErrNotSupported(t *testing.T) {
	var gotErr error
	h := AccessLog()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _, gotErr = http.NewResponseController(w).Hijack()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	if !errors.Is(gotErr, http.ErrNotSupported) {
		t.Fatalf("err = %v, want http.ErrNotSupported", gotErr)
	}
}

// TestWebSocketUpgradeThroughAccessLog 端到端复现 C18：AccessLog 包装后，
// gorilla 的 WebSocket 升级必须成功（修复前返回 500 feature not supported）。
func TestWebSocketUpgradeThroughAccessLog(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(AccessLog()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	})))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("websocket dial failed: %v (status %d)", err, status)
	}
	_ = conn.Close()
}
