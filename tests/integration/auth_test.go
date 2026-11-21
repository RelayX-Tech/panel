package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/config"
	handlers "github.com/relayx-tech/panel/internal/handler/auth"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
)

const testOrigin = "http://panel.test"

func testConfig() config.Config {
	c := config.Config{}
	c.Auth = config.AuthConf{
		SessionCookieName:       "relayx_session",
		SessionIdleTTL:          24 * time.Hour,
		SessionAbsoluteTTL:      168 * time.Hour,
		SessionTouchInterval:    5 * time.Minute,
		CookieSecure:            false,
		AllowedOrigins:          []string{testOrigin},
		LoginRateLimit:          config.LoginRateLimitConf{Window: time.Minute, MaxAttempts: 10},
		PasswordChangeRateLimit: config.PasswordChangeRateLimitConf{Window: 15 * time.Minute, MaxAttempts: 5},
	}
	return c
}

type testEnv struct {
	server *httptest.Server
	svcCtx *svc.ServiceContext
	db     *sql.DB
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	testsupport.RequireRedis(t)
	db := testsupport.NewPostgres(t)
	if err := migrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svcCtx := svc.NewServiceContext(testConfig(), testsupport.Conn(db), testsupport.CacheConf())
	response.InstallErrorHandler()

	loginChain := chain(handlerLogin(svcCtx), svcCtx.OriginCheck, svcCtx.LoginRateLimit)
	logoutChain := chain(handlerLogout(svcCtx), svcCtx.OriginCheck)
	sessionChain := chain(handlerSession(svcCtx), svcCtx.Auth)
	changePasswordChain := chain(handlerChangePassword(svcCtx), svcCtx.OriginCheck, svcCtx.Auth)

	root := middleware.Recovery()(requestid.Middleware()(middleware.AccessLog()(middleware.SecurityHeaders()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
				loginChain(w, r)
			case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/logout":
				logoutChain(w, r)
			case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/session":
				sessionChain(w, r)
			case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/password":
				changePasswordChain(w, r)
			default:
				http.NotFound(w, r)
			}
		})))))

	server := httptest.NewServer(root)
	t.Cleanup(server.Close)
	return &testEnv{server: server, svcCtx: svcCtx, db: db}
}

func chain(h http.HandlerFunc, mws ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func handlerLogin(s *svc.ServiceContext) http.HandlerFunc   { return handlers.LoginHandler(s) }
func handlerLogout(s *svc.ServiceContext) http.HandlerFunc  { return handlers.LogoutHandler(s) }
func handlerSession(s *svc.ServiceContext) http.HandlerFunc { return handlers.SessionHandler(s) }
func handlerChangePassword(s *svc.ServiceContext) http.HandlerFunc {
	return handlers.ChangePasswordHandler(s)
}

func (e *testEnv) createAdmin(t *testing.T, account, password string, disabled bool) *model.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	users := model.NewAuthUserModel(testsupport.Conn(e.db), testsupport.CacheConf())
	u, err := users.Create(context.Background(), nil, account, hash, "Administrator")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if disabled {
		if _, err := e.db.Exec(`UPDATE auth_users SET disabled_at = now() WHERE id = $1::uuid`, u.ID); err != nil {
			t.Fatalf("disable user: %v", err)
		}
		u.DisabledAt = ptrTime(time.Now())
	}
	return u
}

func (e *testEnv) post(t *testing.T, path string, body any, cookie *http.Cookie, origin string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(http.MethodPost, e.server.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := e.server.Client().Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	return resp
}

func (e *testEnv) get(t *testing.T, path string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.server.URL+path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := e.server.Client().Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func cookieFrom(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "relayx_session" {
			return c
		}
	}
	return nil
}

func ptrTime(t time.Time) *time.Time { return &t }

// --- TC-B-101 登录成功 ---

func TestIntegrationLoginSuccess(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "sup3r-secret-pass", false)

	resp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "sup3r-secret-pass"}, nil, testOrigin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decode(t, resp)

	if body["code"] != "OK" || body["message"] != "" {
		t.Errorf("envelope = %#v", body)
	}
	rid, _ := body["requestId"].(string)
	if !regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`).MatchString(rid) {
		t.Errorf("requestId = %q", rid)
	}
	data := body["data"].(map[string]any)
	user := data["user"].(map[string]any)
	if user["account"] != "admin" {
		t.Errorf("user = %#v", user)
	}
	session := data["session"].(map[string]any)
	for _, key := range []string{"expiresAt", "absoluteExpiresAt"} {
		if _, err := time.Parse("2006-01-02T15:04:05.000Z07:00", session[key].(string)); err != nil {
			t.Errorf("%s not RFC3339 ms: %v", key, session[key])
		}
	}

	ck := cookieFrom(resp)
	if ck == nil {
		t.Fatal("session cookie not set")
	}
	if !ck.HttpOnly || ck.Path != "/" || ck.SameSite != http.SameSiteLaxMode || ck.MaxAge != 604800 {
		t.Errorf("cookie attributes = %+v", ck)
	}

	var hash []byte
	if err := env.db.QueryRow(`SELECT session_hash FROM auth_sessions`).Scan(&hash); err != nil {
		t.Fatalf("query session: %v", err)
	}
	if !bytes.Equal(hash, auth.HashSessionToken(ck.Value)) {
		t.Error("stored hash does not match sha256(cookie token)")
	}
	if strings.Contains(string(hash), ck.Value) {
		t.Error("plaintext token stored")
	}
}

// --- TC-B-102 防枚举 ---

func TestIntegrationLoginFailuresAreIdentical(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "correct-password", false)
	env.createAdmin(t, "disabled", "correct-password", true)

	cases := []struct{ account, password string }{
		{"ghost", "correct-password"},
		{"admin", "wrong-password"},
		{"disabled", "correct-password"},
	}
	for _, tc := range cases {
		resp := env.post(t, "/api/v1/auth/login",
			map[string]string{"account": tc.account, "password": tc.password}, nil, testOrigin)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s status = %d", tc.account, resp.StatusCode)
		}
		body := decode(t, resp)
		if body["code"] != "AUTH_INVALID_CREDENTIALS" || body["message"] != "invalid credentials" {
			t.Errorf("%s body = %#v", tc.account, body)
		}
		if body["data"] != nil {
			t.Errorf("%s data = %#v", tc.account, body["data"])
		}
	}
	var n int
	_ = env.db.QueryRow(`SELECT count(*) FROM auth_sessions`).Scan(&n)
	if n != 0 {
		t.Errorf("sessions created for failed logins: %d", n)
	}
}

// --- TC-B-103/104 校验 ---

func TestIntegrationLoginValidation(t *testing.T) {
	env := newTestEnv(t)

	resp := env.post(t, "/api/v1/auth/login", map[string]string{"account": "", "password": "x"}, nil, testOrigin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing account status = %d", resp.StatusCode)
	}
	body := decode(t, resp)
	if body["code"] != "VALIDATION_MISSING_FIELD" || body["field"] != "account" {
		t.Errorf("body = %#v", body)
	}
	if _, ok := body["details"].([]any); !ok {
		t.Errorf("details missing: %#v", body["details"])
	}

	resp = env.post(t, "/api/v1/auth/login",
		map[string]string{"account": strings.Repeat("a", 65), "password": "x"}, nil, testOrigin)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("overlong account status = %d", resp.StatusCode)
	}
	body = decode(t, resp)
	if body["code"] != "VALIDATION_INVALID_FIELD" || body["field"] != "account" {
		t.Errorf("body = %#v", body)
	}

	resp = env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": strings.Repeat("p", 129)}, nil, testOrigin)
	body = decode(t, resp)
	if body["code"] != "VALIDATION_INVALID_FIELD" || body["field"] != "password" {
		t.Errorf("body = %#v", body)
	}
}

// --- TC-B-105 会话查询 ---

func TestIntegrationSessionQuery(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "sup3r-secret-pass", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "sup3r-secret-pass"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	loginBody := decode(t, loginResp)
	loginUser := loginBody["data"].(map[string]any)["user"].(map[string]any)

	resp := env.get(t, "/api/v1/auth/session", ck)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if cookieFrom(resp) != nil {
		t.Error("session query must not reissue cookie")
	}
	body := decode(t, resp)
	user := body["data"].(map[string]any)["user"].(map[string]any)
	if user["id"] != loginUser["id"] || user["account"] != "admin" {
		t.Errorf("user = %#v", user)
	}
}

// --- TC-B-107/108 未认证与过期 ---

func TestIntegrationSessionUnauthorized(t *testing.T) {
	env := newTestEnv(t)

	resp := env.get(t, "/api/v1/auth/session", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no cookie status = %d", resp.StatusCode)
	}
	if body := decode(t, resp); body["code"] != "AUTH_UNAUTHORIZED" {
		t.Errorf("code = %v", body["code"])
	}

	resp = env.get(t, "/api/v1/auth/session", &http.Cookie{Name: "relayx_session", Value: "random-token"})
	if body := decode(t, resp); body["code"] != "AUTH_UNAUTHORIZED" {
		t.Errorf("unknown token code = %v", body["code"])
	}
}

func TestIntegrationSessionExpired(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "sup3r-secret-pass", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "sup3r-secret-pass"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	if _, err := env.db.Exec(`UPDATE auth_sessions SET expires_at = now() - interval '1 minute'`); err != nil {
		t.Fatalf("expire session: %v", err)
	}

	resp := env.get(t, "/api/v1/auth/session", ck)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if body := decode(t, resp); body["code"] != "AUTH_SESSION_EXPIRED" {
		t.Errorf("code = %v", body["code"])
	}
}

// --- TC-B-109/110 注销 ---

func TestIntegrationLogout(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "sup3r-secret-pass", false)

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "sup3r-secret-pass"}, nil, testOrigin)
	ck := cookieFrom(loginResp)
	_ = decode(t, loginResp)

	resp := env.post(t, "/api/v1/auth/logout", nil, ck, testOrigin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d", resp.StatusCode)
	}
	body := decode(t, resp)
	if body["code"] != "OK" || body["data"] != nil {
		t.Errorf("body = %#v", body)
	}
	cleared := cookieFrom(resp)
	if cleared == nil || (cleared.MaxAge != -1 && cleared.MaxAge != 0) {
		t.Errorf("cookie not cleared: %+v", cleared)
	}

	var revoked sql.NullTime
	if err := env.db.QueryRow(`SELECT revoked_at FROM auth_sessions`).Scan(&revoked); err != nil {
		t.Fatalf("query revoked: %v", err)
	}
	if !revoked.Valid {
		t.Error("revoked_at not set")
	}

	if s := env.get(t, "/api/v1/auth/session", ck); s.StatusCode != http.StatusUnauthorized {
		t.Errorf("session after logout status = %d", s.StatusCode)
	} else if b := decode(t, s); b["code"] != "AUTH_SESSION_EXPIRED" {
		t.Errorf("code = %v", b["code"])
	}

	// 幂等：无 Cookie / 已吊销后再注销
	for i := 0; i < 2; i++ {
		r := env.post(t, "/api/v1/auth/logout", nil, nil, testOrigin)
		if r.StatusCode != http.StatusOK {
			t.Errorf("idempotent logout %d status = %d", i, r.StatusCode)
		}
		_ = decode(t, r)
	}
}

// --- TC-B-111 限流 ---

func TestIntegrationLoginRateLimit(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "correct-password", false)

	for i := 0; i < 10; i++ {
		resp := env.post(t, "/api/v1/auth/login",
			map[string]string{"account": "admin", "password": "wrong"}, nil, testOrigin)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", i+1, resp.StatusCode)
		}
		_ = decode(t, resp)
	}
	resp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "wrong"}, nil, testOrigin)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("11th status = %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("Retry-After missing")
	}
	if body := decode(t, resp); body["code"] != "RATE_LIMITED" {
		t.Errorf("code = %v", body["code"])
	}
}

// --- TC-B-112 Origin 校验 ---

func TestIntegrationOriginRejection(t *testing.T) {
	env := newTestEnv(t)
	env.createAdmin(t, "admin", "sup3r-secret-pass", false)

	for _, origin := range []string{"", "https://evil.test"} {
		resp := env.post(t, "/api/v1/auth/login",
			map[string]string{"account": "admin", "password": "sup3r-secret-pass"}, nil, origin)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("origin %q status = %d", origin, resp.StatusCode)
		}
		if body := decode(t, resp); body["code"] != "AUTH_CSRF_REJECTED" {
			t.Errorf("code = %v", body["code"])
		}
	}
}

// --- TC-B-117 格式校验 ---

func TestIntegrationResponseFormats(t *testing.T) {
	env := newTestEnv(t)
	admin := env.createAdmin(t, "admin", "sup3r-secret-pass", false)

	resp := env.get(t, "/api/v1/auth/session", nil)
	_ = decode(t, resp)
	rid := resp.Header.Get("X-Request-Id")
	if !regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`).MatchString(rid) {
		t.Errorf("X-Request-Id = %q", rid)
	}

	loginResp := env.post(t, "/api/v1/auth/login",
		map[string]string{"account": "admin", "password": "sup3r-secret-pass"}, nil, testOrigin)
	body := decode(t, loginResp)
	user := body["data"].(map[string]any)["user"].(map[string]any)
	if user["id"] != admin.ID {
		t.Errorf("user id = %v, want %v", user["id"], admin.ID)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(user["id"].(string)) {
		t.Errorf("user id is not a UUID: %v", user["id"])
	}
}
