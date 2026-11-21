package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
)

// 修改密码请求体解析失败必须落到 400 VALIDATION_INVALID_FIELD，不能穿透成 500。
func TestChangePasswordHandlerRejectsInvalidRequestBody(t *testing.T) {
	response.InstallErrorHandler()

	cases := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"oldPassword":`},
		{name: "empty body", body: ``},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// 解析失败会在触碰 svcCtx 之前返回，空上下文足够。
			ChangePasswordHandler(&svc.ServiceContext{})(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}

			var body response.ErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode envelope: %v; body=%s", err, rec.Body.String())
			}
			if body.Code != "VALIDATION_INVALID_FIELD" || body.Field != "body" {
				t.Fatalf("body = %+v", body)
			}
		})
	}
}

// 合法请求体但上下文无会话（中间件缺失防御）→ 401 AUTH_UNAUTHORIZED。
func TestChangePasswordHandlerRequiresSession(t *testing.T) {
	response.InstallErrorHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password",
		strings.NewReader(`{"oldPassword":"OldPassw0rd!2026","newPassword":"NewPassw0rd!2026"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ChangePasswordHandler(&svc.ServiceContext{})(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	var body response.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if body.Code != "AUTH_UNAUTHORIZED" {
		t.Fatalf("code = %q, want AUTH_UNAUTHORIZED", body.Code)
	}
}
