package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestErrorConstructors(t *testing.T) {
	cases := []struct {
		name       string
		err        *AppError
		wantStatus int
		wantCode   string
	}{
		{"invalid credentials", InvalidCredentials(), http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS"},
		{"unauthorized", Unauthorized(), http.StatusUnauthorized, "AUTH_UNAUTHORIZED"},
		{"session expired", SessionExpired(), http.StatusUnauthorized, "AUTH_SESSION_EXPIRED"},
		{"csrf", CSRFRejected(), http.StatusForbidden, "AUTH_CSRF_REJECTED"},
		{"missing field", MissingField("account"), http.StatusBadRequest, "VALIDATION_MISSING_FIELD"},
		{"invalid field", InvalidField("password", "too long"), http.StatusBadRequest, "VALIDATION_INVALID_FIELD"},
		{"rate limited", RateLimited(30), http.StatusTooManyRequests, "RATE_LIMITED"},
		{"internal", Internal(errors.New("boom")), http.StatusInternalServerError, "INTERNAL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Status != tc.wantStatus || tc.err.Code != tc.wantCode {
				t.Errorf("error = %+v", tc.err)
			}
			if tc.err.Error() == "" {
				t.Error("Error() returned empty string")
			}
		})
	}

	if MissingField("account").Details[0].Code != "REQUIRED" {
		t.Error("missing field details code mismatch")
	}
	if InvalidField("x", "bad").Details[0].Message != "bad" {
		t.Error("invalid field details message mismatch")
	}
	cause := errors.New("boom")
	if Internal(cause).Unwrap() != cause {
		t.Error("Internal does not unwrap cause")
	}
}

func TestErrorHandlerDefaultsAndWrapping(t *testing.T) {
	InstallErrorHandler()

	// AppError 缺省 Status/Code 时回退为 500 INTERNAL
	rec := httptest.NewRecorder()
	httpx.Error(rec, &AppError{Message: "boom"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "INTERNAL" {
		t.Errorf("code = %v", body["code"])
	}

	// 包装错误也能被识别
	wrapped := errors.Join(errors.New("outer"), InvalidCredentials())
	rec = httptest.NewRecorder()
	httpx.Error(rec, wrapped)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrapped status = %d", rec.Code)
	}
}

func TestOKWithoutRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	OK(rec, req, map[string]string{"a": "b"})
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["requestId"] != "" {
		t.Errorf("requestId = %v, want empty", body["requestId"])
	}
}
