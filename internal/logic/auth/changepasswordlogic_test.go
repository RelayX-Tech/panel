package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/config"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

const (
	testUserID = "0b6f1c1e-3a4d-4f0e-9a11-6d2c7b8e9f01"
	testClient = "203.0.113.7"
)

type fakeAudits struct {
	inserted []*model.PasswordAudit
	err      error
}

func (f *fakeAudits) Insert(_ context.Context, _ sqlx.Session, a *model.PasswordAudit) error {
	if f.err != nil {
		return f.err
	}
	cp := *a
	f.inserted = append(f.inserted, &cp)
	return nil
}

func (f *fakeAudits) ListByUserID(context.Context, string, int) ([]model.PasswordAudit, error) {
	return nil, nil
}

func (f *fakeAudits) FindByRequestID(context.Context, string) ([]model.PasswordAudit, error) {
	return nil, nil
}

func (f *fakeAudits) DeleteBefore(context.Context, sqlx.Session, time.Time) (int64, error) {
	return 0, nil
}

type fakeLimiter struct {
	locked     bool
	retryAfter int
	checkErr   error

	recordCount int
	recordRetry int
	recordErr   error

	resetCalled bool
	resetErr    error

	checkCalled  int
	recordCalled int
}

func (f *fakeLimiter) Check(context.Context, string) (bool, int, error) {
	f.checkCalled++
	return f.locked, f.retryAfter, f.checkErr
}

func (f *fakeLimiter) RecordFailure(context.Context, string) (int, int, error) {
	f.recordCalled++
	if f.recordErr != nil {
		return 0, 0, f.recordErr
	}
	return f.recordCount, f.recordRetry, nil
}

func (f *fakeLimiter) Reset(context.Context, string) error {
	f.resetCalled = true
	return f.resetErr
}

// fakeTx 模拟事务：fn 返回错误时回滚用户哈希与会话吊销（design.md §5.1）。
type fakeTx struct {
	users    *fakeUsers
	sessions *fakeSessions
}

func (t *fakeTx) TransactCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	var beforeHash string
	if t.users != nil && t.users.user != nil {
		beforeHash = t.users.user.PasswordHash
	}
	beforeRevoked := 0
	if t.sessions != nil {
		beforeRevoked = t.sessions.revokedAll
	}
	err := fn(ctx, nil)
	if err != nil {
		if t.users != nil && t.users.user != nil {
			t.users.user.PasswordHash = beforeHash
			t.users.updatedHash = ""
		}
		if t.sessions != nil {
			t.sessions.revokedAll = beforeRevoked
		}
	}
	return err
}

type changePasswordEnv struct {
	users    *fakeUsers
	sessions *fakeSessions
	audits   *fakeAudits
	limiter  *fakeLimiter
	svcCtx   *svc.ServiceContext
}

func newChangePasswordEnv(t *testing.T, password string) *changePasswordEnv {
	t.Helper()
	users := &fakeUsers{user: &model.User{
		ID:           testUserID,
		Account:      "admin",
		PasswordHash: mustHash(t, password),
		DisplayName:  "Administrator",
	}}
	sessions := newFakeSessions()
	audits := &fakeAudits{}
	limiter := &fakeLimiter{}
	cfg := config.Config{}
	cfg.Auth = config.AuthConf{
		SessionCookieName:       "relayx_session",
		SessionIdleTTL:          24 * time.Hour,
		SessionAbsoluteTTL:      168 * time.Hour,
		CookieSecure:            true,
		PasswordChangeRateLimit: config.PasswordChangeRateLimitConf{Window: 15 * time.Minute, MaxAttempts: 5},
	}
	svcCtx := &svc.ServiceContext{
		Config:          cfg,
		Tx:              &fakeTx{users: users, sessions: sessions},
		UserModel:       users,
		SessionModel:    sessions,
		AuditModel:      audits,
		PasswordLimiter: limiter,
	}
	return &changePasswordEnv{users: users, sessions: sessions, audits: audits, limiter: limiter, svcCtx: svcCtx}
}

func (e *changePasswordEnv) change(req *types.ChangePasswordReq) (*types.ChangePasswordResp, error) {
	return NewChangePasswordLogic(withRequestID(context.Background()), e.svcCtx).
		ChangePassword(req, testUserID, testClient)
}

func appError(t *testing.T, err error) *response.AppError {
	t.Helper()
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want *response.AppError", err)
	}
	return appErr
}

// TC-BE-001/017/021/027/034：成功更新哈希、吊销全部会话、写审计、清限流并返回 envelope。
func TestChangePassword_Success(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	resp, err := env.change(&types.ChangePasswordReq{
		OldPassword: "OldPassw0rd!2026",
		NewPassword: "NewPassw0rd!2026",
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}

	if resp.Code != "OK" || resp.Message != "" || resp.Data != nil {
		t.Errorf("envelope = %+v", resp)
	}
	if resp.RequestId != "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z" {
		t.Errorf("requestId = %q", resp.RequestId)
	}
	if env.users.updatedHash == "" {
		t.Fatal("password hash not updated")
	}
	ok, err := auth.VerifyPassword("NewPassw0rd!2026", env.users.updatedHash)
	if err != nil || !ok {
		t.Fatalf("new password does not verify: %v", err)
	}
	if env.sessions.revokedAll != 1 {
		t.Errorf("revokedAll = %d, want 1", env.sessions.revokedAll)
	}
	if len(env.audits.inserted) != 1 {
		t.Fatalf("audits = %d, want 1", len(env.audits.inserted))
	}
	audit := env.audits.inserted[0]
	if audit.UserID != testUserID || audit.RequestID != "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z" ||
		audit.ClientIP != testClient || audit.Result != "success" {
		t.Errorf("audit = %+v", audit)
	}
	if !env.users.invalidated {
		t.Error("user cache not invalidated")
	}
	if !env.limiter.resetCalled {
		t.Error("limiter not reset")
	}
}

// TC-BE-002/018/019/020：旧密码错误返回字段级 400，且无副作用、计数 +1。
func TestChangePassword_OldPasswordIncorrect(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.limiter.recordCount = 1
	env.limiter.recordRetry = 900

	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "WrongPassw0rd!",
		NewPassword: "NewPassw0rd!2026",
	})
	appErr := appError(t, err)
	if appErr.Status != 400 || appErr.Code != "VALIDATION_INVALID_OLD_PASSWORD" || appErr.Field != "oldPassword" {
		t.Fatalf("error = %+v", appErr)
	}
	if len(appErr.Details) != 1 || appErr.Details[0].Code != "INVALID_OLD_PASSWORD" {
		t.Errorf("details = %+v", appErr.Details)
	}
	if env.limiter.recordCalled != 1 {
		t.Errorf("recordCalled = %d, want 1", env.limiter.recordCalled)
	}
	if env.users.updatedHash != "" || env.sessions.revokedAll != 0 || len(env.audits.inserted) != 0 {
		t.Error("old-password failure produced side effects")
	}
}

// TC-BE-008：旧密码正确且新旧相同时返回 SAME_AS_OLD，且不计数。
func TestChangePassword_SameAsOld(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "OldPassw0rd!2026",
		NewPassword: "OldPassw0rd!2026",
	})
	appErr := appError(t, err)
	if appErr.Code != "VALIDATION_NEW_PASSWORD_SAME_AS_OLD" || appErr.Field != "newPassword" {
		t.Fatalf("error = %+v", appErr)
	}
	if env.limiter.recordCalled != 0 {
		t.Error("same-as-old attempt counted as failure")
	}
	if env.users.updatedHash != "" {
		t.Error("password updated for same-as-old")
	}
}

// TC-BE-008 反例：旧密码错误且新旧相同 → 优先返回旧密码错误。
func TestChangePassword_OldPasswordWrongWinsOverSameAsOld(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.limiter.recordCount = 1
	env.limiter.recordRetry = 900

	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "WrongPassw0rd!",
		NewPassword: "WrongPassw0rd!",
	})
	if appErr := appError(t, err); appErr.Code != "VALIDATION_INVALID_OLD_PASSWORD" {
		t.Fatalf("error = %+v", appErr)
	}
}

// TC-BE-003~011：字段校验在限流与数据库之前完成。
func TestChangePassword_FieldValidation(t *testing.T) {
	long := strings.Repeat("a", 129)
	cases := []struct {
		name      string
		old       string
		new       string
		wantCode  string
		wantField string
		detail    string
	}{
		{"missing old", "", "NewPassw0rd!2026", "VALIDATION_MISSING_FIELD", "oldPassword", "REQUIRED"},
		{"missing new", "OldPassw0rd!2026", "", "VALIDATION_MISSING_FIELD", "newPassword", "REQUIRED"},
		{"old too long", long, "NewPassw0rd!2026", "VALIDATION_INVALID_FIELD", "oldPassword", "INVALID"},
		{"too short", "OldPassw0rd!2026", strings.Repeat("a", 11), "VALIDATION_INVALID_NEW_PASSWORD", "newPassword", "TOO_SHORT"},
		{"too long", "OldPassw0rd!2026", long, "VALIDATION_INVALID_NEW_PASSWORD", "newPassword", "TOO_LONG"},
		{"all whitespace", "OldPassw0rd!2026", strings.Repeat(" ", 12), "VALIDATION_INVALID_NEW_PASSWORD", "newPassword", "NO_PRINTABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newChangePasswordEnv(t, "OldPassw0rd!2026")
			_, err := env.change(&types.ChangePasswordReq{OldPassword: tc.old, NewPassword: tc.new})
			appErr := appError(t, err)
			if appErr.Status != 400 || appErr.Code != tc.wantCode || appErr.Field != tc.wantField {
				t.Fatalf("error = %+v", appErr)
			}
			if len(appErr.Details) != 1 || appErr.Details[0].Code != tc.detail {
				t.Errorf("details = %+v, want code %s", appErr.Details, tc.detail)
			}
			if env.limiter.checkCalled != 0 {
				t.Error("field validation consulted the limiter")
			}
		})
	}
}

// TC-BE-004/005/015：12 / 128 code point 与 emoji 边界通过。
func TestChangePassword_BoundaryLengthsAccepted(t *testing.T) {
	cases := map[string]string{
		"12 ascii":  strings.Repeat("a", 12),
		"128 ascii": strings.Repeat("a", 128),
		"12 emoji":  strings.Repeat("😀", 12),
	}
	for name, newPassword := range cases {
		t.Run(name, func(t *testing.T) {
			env := newChangePasswordEnv(t, "OldPassw0rd!2026")
			if _, err := env.change(&types.ChangePasswordReq{
				OldPassword: "OldPassw0rd!2026",
				NewPassword: newPassword,
			}); err != nil {
				t.Fatalf("change password: %v", err)
			}
			if env.users.updatedHash == "" {
				t.Error("password not updated")
			}
		})
	}
}

// TC-BE-025：锁定期间直接 429，不查库、不校验旧密码。
func TestChangePassword_LockedSkipsVerify(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.limiter.locked = true
	env.limiter.retryAfter = 42

	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "OldPassw0rd!2026",
		NewPassword: "NewPassw0rd!2026",
	})
	appErr := appError(t, err)
	if appErr.Status != 429 || appErr.Code != "RATE_LIMITED" || appErr.RetryAfterSeconds != 42 {
		t.Fatalf("error = %+v", appErr)
	}
	if env.users.locked {
		t.Error("locked request still queried the user row")
	}
	if env.limiter.checkCalled != 1 {
		t.Errorf("checkCalled = %d, want 1", env.limiter.checkCalled)
	}
}

// TC-BE-024：第 5 次旧密码错误返回 429 + Retry-After。
func TestChangePassword_FifthFailureRateLimited(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.limiter.recordCount = 5
	env.limiter.recordRetry = 900

	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "WrongPassw0rd!",
		NewPassword: "NewPassw0rd!2026",
	})
	appErr := appError(t, err)
	if appErr.Status != 429 || appErr.Code != "RATE_LIMITED" || appErr.RetryAfterSeconds != 900 {
		t.Fatalf("error = %+v", appErr)
	}
}

// TC-BE-029：Redis 不可用时 fail-closed 返回 500 INTERNAL。
func TestChangePassword_LimiterUnavailable(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.limiter.checkErr = errors.New("redis down")

	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "OldPassw0rd!2026",
		NewPassword: "NewPassw0rd!2026",
	})
	if appErr := appError(t, err); appErr.Status != 500 || appErr.Code != "INTERNAL" {
		t.Fatalf("error = %+v", appErr)
	}
}

// TC-BE-031：审计写入失败时整体回滚（哈希不变、会话不吊销）。
func TestChangePassword_AuditFailureRollsBack(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.audits.err = errors.New("audit insert failed")

	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "OldPassw0rd!2026",
		NewPassword: "NewPassw0rd!2026",
	})
	if appErr := appError(t, err); appErr.Status != 500 || appErr.Code != "INTERNAL" {
		t.Fatalf("error = %+v", appErr)
	}
	if env.users.updatedHash != "" {
		t.Error("password hash changed despite rollback")
	}
	if env.sessions.revokedAll != 0 {
		t.Error("sessions revoked despite rollback")
	}
}

// TC-BE-012/013：用户不存在（会话指向已删除账号）返回 401 AUTH_SESSION_EXPIRED。
func TestChangePassword_UserMissing(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.users.user = nil

	_, err := env.change(&types.ChangePasswordReq{
		OldPassword: "OldPassw0rd!2026",
		NewPassword: "NewPassw0rd!2026",
	})
	if appErr := appError(t, err); appErr.Status != 401 || appErr.Code != "AUTH_SESSION_EXPIRED" {
		t.Fatalf("error = %+v", appErr)
	}
}

// 缓存失效 / 限流清零失败不影响已提交的成功结果（best-effort）。
func TestChangePassword_PostCommitFailuresAreNonFatal(t *testing.T) {
	env := newChangePasswordEnv(t, "OldPassw0rd!2026")
	env.users.invalidateErr = errors.New("cache down")
	env.limiter.resetErr = errors.New("redis down")

	resp, err := env.change(&types.ChangePasswordReq{
		OldPassword: "OldPassw0rd!2026",
		NewPassword: "NewPassw0rd!2026",
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	if resp.Code != "OK" {
		t.Errorf("envelope = %+v", resp)
	}
}
