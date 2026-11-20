package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeUsers struct {
	user  *model.User
	err   error
	users []*model.User
}

func (f *fakeUsers) Create(_ context.Context, account, hash, display string) (*model.User, error) {
	return &model.User{Account: account, PasswordHash: hash, DisplayName: display}, nil
}

func (f *fakeUsers) FindByAccount(context.Context, string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.user == nil {
		return nil, model.ErrNotFound
	}
	cp := *f.user
	return &cp, nil
}

func (f *fakeUsers) Count(context.Context) (int64, error) { return 0, nil }

type fakeSessions struct {
	created []*model.Session
	rows    map[string]*model.SessionWithUser
	touched int
	revoked int
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{rows: map[string]*model.SessionWithUser{}}
}

func (f *fakeSessions) Create(_ context.Context, s *model.Session) error {
	s.ID = "sess-1"
	f.created = append(f.created, s)
	return nil
}

func (f *fakeSessions) FindByHash(_ context.Context, hash []byte) (*model.SessionWithUser, error) {
	row, ok := f.rows[string(hash)]
	if !ok {
		return nil, model.ErrNotFound
	}
	cp := *row
	return &cp, nil
}

func (f *fakeSessions) Touch(context.Context, string, time.Time, time.Time) error {
	f.touched++
	return nil
}

func (f *fakeSessions) RevokeByHash(context.Context, []byte) error {
	f.revoked++
	return nil
}

func (f *fakeSessions) DeleteExpiredBefore(context.Context, time.Time) (int64, error) { return 0, nil }

func newAuthSvc(t *testing.T, users *fakeUsers) (*svc.ServiceContext, *fakeSessions) {
	t.Helper()
	sessions := newFakeSessions()
	cfg := config.Config{}
	cfg.Auth = config.AuthConf{
		SessionCookieName:  "relayx_session",
		SessionIdleTTL:     24 * time.Hour,
		SessionAbsoluteTTL: 168 * time.Hour,
		CookieSecure:       true,
	}
	manager := auth.NewSessionManager(sessions, 24*time.Hour, 168*time.Hour, 5*time.Minute)
	return &svc.ServiceContext{Config: cfg, UserModel: users, SessionManager: manager}, sessions
}

func withRequestID(ctx context.Context) context.Context {
	return requestid.NewContext(ctx, "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z")
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return h
}

func TestLoginSuccess(t *testing.T) {
	users := &fakeUsers{user: &model.User{
		ID: "0b6f1c1e-3a4d-4f0e-9a11-6d2c7b8e9f01", Account: "admin",
		PasswordHash: mustHash(t, "sup3r-secret-pass"), DisplayName: "Administrator",
	}}
	svcCtx, sessions := newAuthSvc(t, users)

	l := NewLoginLogic(withRequestID(context.Background()), svcCtx)
	resp, token, err := l.Login(&types.LoginReq{Account: "admin", Password: "sup3r-secret-pass"}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if token == "" || len(token) != 43 {
		t.Errorf("token = %q", token)
	}
	if len(sessions.created) != 1 {
		t.Fatalf("sessions created = %d, want 1", len(sessions.created))
	}
	if resp.Code != "OK" || resp.Message != "" || resp.RequestId != "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z" {
		t.Errorf("envelope = %+v", resp)
	}
	if resp.Data.User.Id != users.user.ID || resp.Data.User.Account != "admin" || resp.Data.User.DisplayName != "Administrator" {
		t.Errorf("user = %+v", resp.Data.User)
	}
	if _, err := time.Parse("2006-01-02T15:04:05.000Z07:00", resp.Data.Session.ExpiresAt); err != nil {
		t.Errorf("expiresAt not RFC3339 ms: %q", resp.Data.Session.ExpiresAt)
	}
	if _, err := time.Parse("2006-01-02T15:04:05.000Z07:00", resp.Data.Session.AbsoluteExpiresAt); err != nil {
		t.Errorf("absoluteExpiresAt not RFC3339 ms: %q", resp.Data.Session.AbsoluteExpiresAt)
	}
}

func TestLoginTrimsAccountNotPassword(t *testing.T) {
	users := &fakeUsers{user: &model.User{
		ID: "u1", Account: "admin", PasswordHash: mustHash(t, " pass with spaces "),
	}}
	svcCtx, _ := newAuthSvc(t, users)
	l := NewLoginLogic(context.Background(), svcCtx)

	// 密码不 trim：带空格必须原样匹配
	if _, _, err := l.Login(&types.LoginReq{Account: "  admin  ", Password: " pass with spaces "}, "", ""); err != nil {
		t.Fatalf("login with padded account failed: %v", err)
	}
	if _, _, err := l.Login(&types.LoginReq{Account: "admin", Password: "pass with spaces"}, "", ""); err == nil {
		t.Fatal("password was trimmed")
	}
}

func TestLoginValidation(t *testing.T) {
	users := &fakeUsers{}
	svcCtx, _ := newAuthSvc(t, users)
	l := NewLoginLogic(context.Background(), svcCtx)

	cases := []struct {
		name      string
		req       types.LoginReq
		wantCode  string
		wantField string
	}{
		{"empty account", types.LoginReq{Account: "", Password: "x"}, "VALIDATION_MISSING_FIELD", "account"},
		{"blank account", types.LoginReq{Account: "   ", Password: "x"}, "VALIDATION_MISSING_FIELD", "account"},
		{"empty password", types.LoginReq{Account: "admin", Password: ""}, "VALIDATION_MISSING_FIELD", "password"},
		{"account too long", types.LoginReq{Account: repeat("a", 65), Password: "x"}, "VALIDATION_INVALID_FIELD", "account"},
		{"password too long", types.LoginReq{Account: "admin", Password: repeat("p", 129)}, "VALIDATION_INVALID_FIELD", "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := l.Login(&tc.req, "", "")
			var appErr *response.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("error = %v", err)
			}
			if appErr.Code != tc.wantCode || appErr.Field != tc.wantField {
				t.Errorf("error = %+v", appErr)
			}
			if appErr.Status != 400 {
				t.Errorf("status = %d, want 400", appErr.Status)
			}
		})
	}
}

func TestLoginBoundaryLengthsAccepted(t *testing.T) {
	users := &fakeUsers{err: model.ErrNotFound}
	svcCtx, _ := newAuthSvc(t, users)
	l := NewLoginLogic(context.Background(), svcCtx)

	// 64 / 128 字符为合法边界（用户不存在 → 凭证错误，而不是校验错误）
	_, _, err := l.Login(&types.LoginReq{Account: repeat("a", 64), Password: repeat("p", 128)}, "", "")
	var appErr *response.AppError
	if !errors.As(err, &appErr) || appErr.Code != "AUTH_INVALID_CREDENTIALS" {
		t.Fatalf("error = %v, want invalid credentials", err)
	}
}

func TestLoginInvalidCredentialsLookIdentical(t *testing.T) {
	password := "correct-password"
	cases := map[string]*fakeUsers{
		"unknown account": {err: model.ErrNotFound},
		"wrong password":  {user: &model.User{ID: "u1", Account: "admin", PasswordHash: mustHash(t, password)}},
		"disabled account": {user: &model.User{ID: "u1", Account: "admin", PasswordHash: mustHash(t, password),
			DisabledAt: ptrTime(time.Now())}},
	}
	for name, users := range cases {
		t.Run(name, func(t *testing.T) {
			svcCtx, sessions := newAuthSvc(t, users)
			l := NewLoginLogic(withRequestID(context.Background()), svcCtx)

			resp, token, err := l.Login(&types.LoginReq{Account: "admin", Password: "wrong-password"}, "", "")
			if resp != nil || token != "" {
				t.Errorf("response leaked on failure: %+v", resp)
			}
			var appErr *response.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("error = %v", err)
			}
			if appErr.Status != 401 || appErr.Code != "AUTH_INVALID_CREDENTIALS" || appErr.Message != "invalid credentials" {
				t.Errorf("error = %+v", appErr)
			}
			if len(sessions.created) != 0 {
				t.Error("session created for failed login")
			}
		})
	}
}

func TestLoginInternalError(t *testing.T) {
	users := &fakeUsers{err: errors.New("db connection refused")}
	svcCtx, _ := newAuthSvc(t, users)
	l := NewLoginLogic(withRequestID(context.Background()), svcCtx)

	_, _, err := l.Login(&types.LoginReq{Account: "admin", Password: "x"}, "", "")
	var appErr *response.AppError
	if !errors.As(err, &appErr) || appErr.Code != "INTERNAL" {
		t.Fatalf("error = %v, want INTERNAL", err)
	}
	if appErr.Message == "db connection refused" {
		t.Error("internal error detail leaked in message")
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func ptrTime(t time.Time) *time.Time { return &t }
