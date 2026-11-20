package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
)

// --- Auth middleware ---

type stubManager struct {
	row *model.SessionWithUser
	err error
}

func (s *stubManager) Authenticate(context.Context, string) (*model.SessionWithUser, error) {
	return s.row, s.err
}

func TestAuthMiddlewareAllowsValidSession(t *testing.T) {
	response.InstallErrorHandler()
	mw := NewAuthMiddleware(&stubManager{row: &model.SessionWithUser{
		User: model.User{ID: "u1", Account: "admin"},
	}}, "relayx_session")

	var seen *model.SessionWithUser
	h := mw.Handle(func(w http.ResponseWriter, r *http.Request) {
		seen = auth.SessionFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: "relayx_session", Value: "token"})
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if seen == nil || seen.User.ID != "u1" {
		t.Fatalf("session not injected: %+v", seen)
	}
}

func TestAuthMiddlewareRejectsMissingAndExpired(t *testing.T) {
	response.InstallErrorHandler()
	cases := []struct {
		name     string
		manager  *stubManager
		cookie   bool
		wantCode string
	}{
		{"no cookie", &stubManager{err: auth.ErrSessionNotFound}, false, "AUTH_UNAUTHORIZED"},
		{"unknown token", &stubManager{err: auth.ErrSessionNotFound}, true, "AUTH_UNAUTHORIZED"},
		{"expired", &stubManager{err: auth.ErrSessionExpired}, true, "AUTH_SESSION_EXPIRED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mw := NewAuthMiddleware(tc.manager, "relayx_session")
			called := false
			h := mw.Handle(func(http.ResponseWriter, *http.Request) { called = true })

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
			if tc.cookie {
				req.AddCookie(&http.Cookie{Name: "relayx_session", Value: "token"})
			}
			rec := httptest.NewRecorder()
			h(rec, req)

			if called {
				t.Fatal("next handler called for invalid session")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			var body map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if body["code"] != tc.wantCode {
				t.Errorf("code = %v, want %s", body["code"], tc.wantCode)
			}
		})
	}
}

// --- Origin middleware ---

func TestOriginCheck(t *testing.T) {
	response.InstallErrorHandler()
	mw := NewOriginCheckMiddleware([]string{"https://panel.example.com"})

	cases := []struct {
		name       string
		method     string
		origin     string
		fetchSite  string
		wantStatus int
	}{
		{"allowed origin", http.MethodPost, "https://panel.example.com", "", http.StatusOK},
		{"missing origin", http.MethodPost, "", "", http.StatusForbidden},
		{"evil origin", http.MethodPost, "https://evil.test", "", http.StatusForbidden},
		{"cross site", http.MethodPost, "https://panel.example.com", "cross-site", http.StatusForbidden},
		{"same origin", http.MethodPost, "https://panel.example.com", "same-origin", http.StatusOK},
		{"get bypasses", http.MethodGet, "", "", http.StatusOK},
		{"origin prefix not matched", http.MethodPost, "https://panel.example.com.evil.test", "", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := mw.Handle(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			req := httptest.NewRequest(tc.method, "/api/v1/auth/login", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			}
			rec := httptest.NewRecorder()
			h(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusForbidden {
				var body map[string]any
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if body["code"] != "AUTH_CSRF_REJECTED" {
					t.Errorf("code = %v", body["code"])
				}
			}
		})
	}
}

func TestRemoteIPHonorsTrustedProxies(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.1.2.3:5000"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.1.2.3")

	if got := RemoteIP(req, false, nil); got != "10.1.2.3" {
		t.Errorf("untrusted = %q, want connection IP", got)
	}
	if got := RemoteIP(req, true, []string{"192.0.2.0/24"}); got != "10.1.2.3" {
		t.Errorf("non-matching proxy CIDR = %q, want connection IP", got)
	}
	if got := RemoteIP(req, true, []string{"10.0.0.0/8"}); got != "203.0.113.9" {
		t.Errorf("trusted proxy = %q, want XFF client IP", got)
	}
}

// --- Login rate limit middleware ---

func TestLoginRateLimit(t *testing.T) {
	response.InstallErrorHandler()
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	mw := NewLoginRateLimitMiddleware(60*time.Second, 10, "/api/v1/auth/login")
	mw.Now = func() time.Time { return now }

	h := mw.Handle(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	do := func(path, ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	for i := 0; i < 10; i++ {
		if rec := do("/api/v1/auth/login", "203.0.113.7"); rec.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d", i+1, rec.Code)
		}
	}
	rec := do("/api/v1/auth/login", "203.0.113.7")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("11th attempt status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rec.Header().Get("Retry-After"))
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "RATE_LIMITED" {
		t.Errorf("code = %v", body["code"])
	}

	// 其他 IP 独立计数
	if rec := do("/api/v1/auth/login", "203.0.113.8"); rec.Code != http.StatusOK {
		t.Errorf("other ip status = %d", rec.Code)
	}
	// 非登录路径不限流
	for i := 0; i < 20; i++ {
		if rec := do("/api/v1/auth/logout", "203.0.113.7"); rec.Code != http.StatusOK {
			t.Fatalf("logout attempt %d status = %d", i+1, rec.Code)
		}
	}

	// 窗口过后恢复
	now = now.Add(61 * time.Second)
	if rec := do("/api/v1/auth/login", "203.0.113.7"); rec.Code != http.StatusOK {
		t.Errorf("after window status = %d", rec.Code)
	}
}
