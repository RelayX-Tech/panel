package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/redis"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/testsupport"
)

func redisClient(t *testing.T) *redis.Redis {
	t.Helper()
	testsupport.RequireRedis(t)
	return redis.RedisConf{Host: testsupport.RedisAddr(), Type: "node"}.NewRedis()
}

func (e *testEnv) changePassword(t *testing.T, oldPassword, newPassword string, cookie *http.Cookie) *http.Response {
	t.Helper()
	return e.post(t, "/api/v1/auth/password",
		map[string]string{"oldPassword": oldPassword, "newPassword": newPassword}, cookie, testOrigin)
}

// TC-BE-001/017/021/022/034/037/039/041/043：成功修改的完整副作用。
func TestChangePasswordIntegrationSuccess(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin(t, "admin", "OldPassw0rd!2026", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	resp := env.changePassword(t, "OldPassw0rd!2026", "NewPassw0rd!2026", ck)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; body=%s", resp.StatusCode, readBody(resp))
	}
	body := decode(t, resp)
	if body["code"] != "OK" || body["message"] != "" || body["data"] != nil {
		t.Errorf("envelope = %#v", body)
	}
	rid, _ := body["requestId"].(string)
	if rid == "" || resp.Header.Get("X-Request-Id") != rid {
		t.Errorf("requestId = %q, header = %q", rid, resp.Header.Get("X-Request-Id"))
	}
	cleared := cookieFrom(resp)
	if cleared == nil || (cleared.MaxAge != -1 && cleared.MaxAge != 0) {
		t.Errorf("session cookie not cleared: %+v", cleared)
	}

	var hash string
	if err := env.db.QueryRow(`SELECT password_hash FROM auth_users WHERE id = $1::uuid`, admin.ID).Scan(&hash); err != nil {
		t.Fatalf("query hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash = %q, want Argon2id PHC", hash)
	}
	if ok, err := auth.VerifyPassword("NewPassw0rd!2026", hash); err != nil || !ok {
		t.Fatalf("new password does not verify: %v", err)
	}

	var active int
	if err := env.db.QueryRow(`SELECT count(*) FROM auth_sessions WHERE user_id = $1::uuid AND revoked_at IS NULL`, admin.ID).Scan(&active); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if active != 0 {
		t.Errorf("active sessions = %d, want 0", active)
	}

	var auditUser, auditRequest, auditIP, auditResult string
	if err := env.db.QueryRow(`SELECT user_id::text, request_id, coalesce(host(client_ip), ''), result
FROM auth_password_audits`).Scan(&auditUser, &auditRequest, &auditIP, &auditResult); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditUser != admin.ID || auditRequest != rid || auditResult != "success" || auditIP == "" {
		t.Errorf("audit = %q %q %q %q", auditUser, auditRequest, auditIP, auditResult)
	}

	// 成功修改后限流计数清零。
	if v, err := redisClient(t).GetCtx(context.Background(), auth.PasswordFailureKeyPrefix+admin.ID); err != nil || v != "" {
		t.Errorf("limiter key = %q, %v; want deleted", v, err)
	}

	// 当前会话已吊销；旧密码不可登录、新密码可登录。
	after := env.get(t, "/api/v1/auth/session", ck)
	if after.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session after change status = %d", after.StatusCode)
	}
	if b := decode(t, after); b["code"] != "AUTH_SESSION_EXPIRED" {
		t.Errorf("code = %v", b["code"])
	}
	if resp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d", resp.StatusCode)
	} else {
		_ = decode(t, resp)
	}
	if resp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "NewPassw0rd!2026"}, nil, testOrigin); resp.StatusCode != http.StatusOK {
		t.Fatalf("new password login status = %d; body=%s", resp.StatusCode, readBody(resp))
	} else {
		_ = decode(t, resp)
	}
}

// TC-BE-002/019/020/023：旧密码错误字段级 400，无副作用，计数 +1，会话仍有效。
func TestChangePasswordIntegrationOldPasswordError(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin(t, "admin", "OldPassw0rd!2026", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	var before string
	if err := env.db.QueryRow(`SELECT password_hash FROM auth_users WHERE id = $1::uuid`, admin.ID).Scan(&before); err != nil {
		t.Fatalf("query hash: %v", err)
	}

	resp := env.changePassword(t, "WrongPassw0rd!", "NewPassw0rd!2026", ck)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d; body=%s", resp.StatusCode, readBody(resp))
	}
	body := decode(t, resp)
	if body["code"] != "VALIDATION_INVALID_OLD_PASSWORD" || body["field"] != "oldPassword" {
		t.Errorf("body = %#v", body)
	}
	details, _ := body["details"].([]any)
	if len(details) != 1 {
		t.Fatalf("details = %#v", body["details"])
	}
	if detail := details[0].(map[string]any); detail["code"] != "INVALID_OLD_PASSWORD" {
		t.Errorf("detail = %#v", detail)
	}

	var after string
	if err := env.db.QueryRow(`SELECT password_hash FROM auth_users WHERE id = $1::uuid`, admin.ID).Scan(&after); err != nil {
		t.Fatalf("query hash: %v", err)
	}
	if after != before {
		t.Error("password hash changed on old-password failure")
	}
	var revoked int
	if err := env.db.QueryRow(`SELECT count(*) FROM auth_sessions WHERE user_id = $1::uuid AND revoked_at IS NOT NULL`, admin.ID).Scan(&revoked); err != nil {
		t.Fatalf("count revoked: %v", err)
	}
	if revoked != 0 {
		t.Errorf("revoked sessions = %d, want 0", revoked)
	}
	var audits int
	if err := env.db.QueryRow(`SELECT count(*) FROM auth_password_audits`).Scan(&audits); err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 0 {
		t.Errorf("audits = %d, want 0", audits)
	}
	if v, err := redisClient(t).GetCtx(context.Background(), auth.PasswordFailureKeyPrefix+admin.ID); err != nil || v != "1" {
		t.Errorf("limiter count = %q, %v; want 1", v, err)
	}

	// 会话仍有效（AC21）：下一次性请求不触发全局 401。
	if s := env.get(t, "/api/v1/auth/session", ck); s.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200", s.StatusCode)
	} else {
		_ = decode(t, s)
	}
}

// TC-BE-023/024/025/028：5 次旧密码错误后锁定；按用户独立；登录限流不受影响。
func TestChangePasswordIntegrationRateLimit(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "OldPassw0rd!2026", false)
	other := env.createAdmin(t, "other", "OtherPassw0rd!2026", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	for i := 1; i <= 4; i++ {
		resp := env.changePassword(t, "WrongPassw0rd!", "NewPassw0rd!2026", ck)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("attempt %d status = %d; body=%s", i, resp.StatusCode, readBody(resp))
		}
		_ = decode(t, resp)
	}
	resp := env.changePassword(t, "WrongPassw0rd!", "NewPassw0rd!2026", ck)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("5th status = %d, want 429; body=%s", resp.StatusCode, readBody(resp))
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("Retry-After missing")
	}
	if body := decode(t, resp); body["code"] != "RATE_LIMITED" {
		t.Errorf("code = %v", body["code"])
	}

	// 锁定期间即使旧密码正确也直接 429（不校验旧密码）。
	if resp := env.changePassword(t, "OldPassw0rd!2026", "NewPassw0rd!2026", ck); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("locked status = %d, want 429", resp.StatusCode)
	} else {
		_ = decode(t, resp)
	}

	// 按用户独立：另一个用户首次错误仍为 400。
	otherLogin := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "other", "password": "OtherPassw0rd!2026"}, nil, testOrigin)
	otherCookie := cookieFrom(otherLogin)
	_ = decode(t, otherLogin)
	if resp := env.changePassword(t, "WrongPassw0rd!", "NewPassw0rd!2026", otherCookie); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("other user status = %d, want 400", resp.StatusCode)
	} else {
		_ = decode(t, resp)
	}
	if v, _ := redisClient(t).GetCtx(context.Background(), auth.PasswordFailureKeyPrefix+other.ID); v != "1" {
		t.Errorf("other limiter count = %q, want 1", v)
	}

	// 登录限流与修改密码限流独立：正确密码仍可登录。
	if resp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin); resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (independent limiter)", resp.StatusCode)
	} else {
		_ = decode(t, resp)
	}
}

// TC-BE-012/014/048：无会话 401；非白名单 Origin 403。
func TestChangePasswordIntegrationAuthAndCSRF(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "OldPassw0rd!2026", false)

	resp := env.post(t, "/api/v1/auth/password",
		map[string]string{"oldPassword": "OldPassw0rd!2026", "newPassword": "NewPassw0rd!2026"}, nil, testOrigin)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no cookie status = %d", resp.StatusCode)
	}
	if body := decode(t, resp); body["code"] != "AUTH_UNAUTHORIZED" {
		t.Errorf("code = %v", body["code"])
	}

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	resp = env.post(t, "/api/v1/auth/password",
		map[string]string{"oldPassword": "OldPassw0rd!2026", "newPassword": "NewPassw0rd!2026"}, ck, "https://evil.test")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bad origin status = %d", resp.StatusCode)
	}
	if body := decode(t, resp); body["code"] != "AUTH_CSRF_REJECTED" {
		t.Errorf("code = %v", body["code"])
	}
}

// TC-BE-003/006/007/008/016：新密码不合规 / 新旧相同均无副作用。
func TestChangePasswordIntegrationInvalidNewPassword(t *testing.T) {
	cases := []struct {
		name       string
		newPass    string
		wantCode   string
		wantDetail string
	}{
		{"too short", strings.Repeat("a", 11), "VALIDATION_INVALID_NEW_PASSWORD", "TOO_SHORT"},
		{"too long", strings.Repeat("a", 129), "VALIDATION_INVALID_NEW_PASSWORD", "TOO_LONG"},
		{"all whitespace", strings.Repeat(" ", 12), "VALIDATION_INVALID_NEW_PASSWORD", "NO_PRINTABLE"},
		{"same as old", "OldPassw0rd!2026", "VALIDATION_NEW_PASSWORD_SAME_AS_OLD", "SAME_AS_OLD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			admin := env.createAdmin(t, "admin", "OldPassw0rd!2026", false)
			loginResp := env.post(t, "/api/v1/auth/login",
				map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin)
			ck := cookieFrom(loginResp)
			_ = decode(t, loginResp)

			resp := env.changePassword(t, "OldPassw0rd!2026", tc.newPass, ck)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d; body=%s", resp.StatusCode, readBody(resp))
			}
			body := decode(t, resp)
			if body["code"] != tc.wantCode || body["field"] != "newPassword" {
				t.Errorf("body = %#v", body)
			}
			details, _ := body["details"].([]any)
			if len(details) != 1 || details[0].(map[string]any)["code"] != tc.wantDetail {
				t.Errorf("details = %#v, want %s", body["details"], tc.wantDetail)
			}
			var audits int
			if err := env.db.QueryRow(`SELECT count(*) FROM auth_password_audits`).Scan(&audits); err != nil {
				t.Fatalf("count audits: %v", err)
			}
			if audits != 0 {
				t.Errorf("audits = %d, want 0", audits)
			}
			if v, _ := redisClient(t).GetCtx(context.Background(), auth.PasswordFailureKeyPrefix+admin.ID); v != "" {
				t.Errorf("limiter count = %q, want empty", v)
			}
		})
	}
}

// TC-BE-032/035：并发修改只允许一个成功，且只产生一行审计。
func TestChangePasswordIntegrationConcurrentOnlyOneSucceeds(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "OldPassw0rd!2026", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	statuses := make([]int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i] = postChangePasswordRaw(t, env.server.URL, ck, "OldPassw0rd!2026", "NewPassw0rd!2026")
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, status := range statuses {
		if status == http.StatusOK {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d, want 1 (statuses %v)", successes, statuses)
	}
	var audits int
	if err := env.db.QueryRow(`SELECT count(*) FROM auth_password_audits`).Scan(&audits); err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 1 {
		t.Errorf("audits = %d, want 1", audits)
	}
}

// TC-BE-040/041：X-Request-Id 透传到响应头、响应体与审计。
func TestChangePasswordIntegrationRequestIDPropagation(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin(t, "admin", "OldPassw0rd!2026", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "OldPassw0rd!2026"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	requestID := "6f1c1e3a-4d4f-4e0a-9a11-6d2c7b8e9f01"
	body, _ := json.Marshal(map[string]string{
		"oldPassword": "OldPassw0rd!2026",
		"newPassword": "NewPassw0rd!2026",
	})
	req, err := http.NewRequest(http.MethodPost, env.server.URL+"/api/v1/auth/password", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	req.Header.Set(requestIDHeader, requestID)
	req.AddCookie(ck)
	resp, err := env.server.Client().Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; body=%s", resp.StatusCode, readBody(resp))
	}
	if got := resp.Header.Get(requestIDHeader); got != requestID {
		t.Errorf("response header = %q, want %q", got, requestID)
	}
	envelope := decode(t, resp)
	if envelope["requestId"] != requestID {
		t.Errorf("body requestId = %v", envelope["requestId"])
	}

	var auditRequest string
	if err := env.db.QueryRow(`SELECT request_id FROM auth_password_audits WHERE user_id = $1::uuid`, admin.ID).Scan(&auditRequest); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditRequest != requestID {
		t.Errorf("audit request_id = %q, want %q", auditRequest, requestID)
	}
}

const requestIDHeader = "X-Request-Id"

func postChangePasswordRaw(t *testing.T, serverURL string, cookie *http.Cookie, oldPassword, newPassword string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"oldPassword": oldPassword, "newPassword": newPassword})
	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/v1/auth/password", bytes.NewReader(body))
	if err != nil {
		return -1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func readBody(resp *http.Response) string {
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.String()
}
