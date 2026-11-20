package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzAlwaysOK(t *testing.T) {
	rec := httptest.NewRecorder()
	Healthz().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != `{"status":"ok"}` {
		t.Errorf("body = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestReadyzReportsDependencyStatus(t *testing.T) {
	ok := func() error { return nil }
	rec := httptest.NewRecorder()
	Readyz(ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", rec.Code)
	}

	failing := func() error { return errors.New("db down") }
	rec = httptest.NewRecorder()
	Readyz(failing).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("not-ready status = %d, want 503", rec.Code)
	}
	// 失败时不得泄露内部错误细节
	if body := rec.Body.String(); body == "" || body == `{"error":"db down"}` {
		t.Errorf("body = %q, want generic status payload", body)
	}
}

func TestReadyzChecksAllDependencies(t *testing.T) {
	ok := func() error { return nil }
	failing := func() error { return errors.New("redis down") }

	rec := httptest.NewRecorder()
	Readyz(ok, failing).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when any dependency fails", rec.Code)
	}

	rec = httptest.NewRecorder()
	Readyz(ok, ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when all dependencies are healthy", rec.Code)
	}
}
