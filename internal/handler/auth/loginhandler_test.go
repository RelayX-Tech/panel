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

// 登录请求体解析失败必须落到 400 VALIDATION_INVALID_FIELD（design.md §3.6），
// 不能穿透成 500 INTERNAL。
func TestLoginHandlerRejectsInvalidRequestBody(t *testing.T) {
	response.InstallErrorHandler()

	cases := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"account":`},
		{name: "empty body", body: ``},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// 解析失败会在触碰 svcCtx 之前返回，空上下文足够。
			LoginHandler(&svc.ServiceContext{})(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}

			var body response.ErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode envelope: %v; body=%s", err, rec.Body.String())
			}
			if body.Code != "VALIDATION_INVALID_FIELD" {
				t.Fatalf("code = %q, want VALIDATION_INVALID_FIELD", body.Code)
			}
			if body.Field != "body" {
				t.Fatalf("field = %q, want %q", body.Field, "body")
			}
		})
	}
}
