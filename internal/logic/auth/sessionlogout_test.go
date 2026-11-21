package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
)

type failingSessions struct{ fakeSessions }

func (f *failingSessions) RevokeByHash(context.Context, sqlx.Session, []byte) error {
	return errors.New("db unavailable")
}

func TestSessionLogicReturnsCurrentSession(t *testing.T) {
	svcCtx, _ := newAuthSvc(t, &fakeUsers{})
	expires := time.Date(2026, 11, 20, 8, 0, 0, 500*int(time.Millisecond), time.UTC)
	absolute := time.Date(2026, 11, 26, 8, 0, 0, 0, time.UTC)
	row := &model.SessionWithUser{
		Session: model.Session{ExpiresAt: expires, AbsoluteExpiresAt: absolute},
		User:    model.User{ID: "u1", Account: "admin", DisplayName: "Administrator"},
	}
	ctx := auth.ContextWithSession(withRequestID(context.Background()), row)

	resp, err := NewSessionLogic(ctx, svcCtx).Session()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if resp.Code != "OK" || resp.RequestId != "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z" {
		t.Errorf("envelope = %+v", resp)
	}
	if resp.Data.User.Id != "u1" || resp.Data.User.Account != "admin" {
		t.Errorf("user = %+v", resp.Data.User)
	}
	if resp.Data.Session.ExpiresAt != "2026-11-20T08:00:00.500Z" {
		t.Errorf("expiresAt = %q", resp.Data.Session.ExpiresAt)
	}
	if resp.Data.Session.AbsoluteExpiresAt != "2026-11-26T08:00:00.000Z" {
		t.Errorf("absoluteExpiresAt = %q", resp.Data.Session.AbsoluteExpiresAt)
	}
}

func TestSessionLogicWithoutSessionFailsClosed(t *testing.T) {
	svcCtx, _ := newAuthSvc(t, &fakeUsers{})
	_, err := NewSessionLogic(context.Background(), svcCtx).Session()
	var appErr *response.AppError
	if !errors.As(err, &appErr) || appErr.Code != "AUTH_UNAUTHORIZED" {
		t.Fatalf("error = %v", err)
	}
}

func TestLogoutLogicRevokesAndIsIdempotent(t *testing.T) {
	svcCtx, sessions := newAuthSvc(t, &fakeUsers{})
	l := NewLogoutLogic(withRequestID(context.Background()), svcCtx)

	resp, err := l.Logout("some-token")
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if resp.Code != "OK" || resp.Data != nil || resp.RequestId != "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z" {
		t.Errorf("envelope = %+v", resp)
	}
	if sessions.revoked != 1 {
		t.Errorf("revoked = %d, want 1", sessions.revoked)
	}

	// 无 Cookie 也返回成功（幂等），不触发吊销
	resp, err = l.Logout("")
	if err != nil || resp.Code != "OK" {
		t.Errorf("idempotent logout = %+v, %v", resp, err)
	}
	if sessions.revoked != 1 {
		t.Errorf("revoked after empty logout = %d, want 1", sessions.revoked)
	}
}

func TestLogoutLogicInternalError(t *testing.T) {
	svcCtx, _ := newAuthSvc(t, &fakeUsers{})
	failing := &failingSessions{fakeSessions: *newFakeSessions()}
	svcCtx.SessionManager = auth.NewSessionManager(failing, 24*time.Hour, 168*time.Hour, 5*time.Minute)

	l := NewLogoutLogic(context.Background(), svcCtx)
	_, err := l.Logout("some-token")
	var appErr *response.AppError
	if !errors.As(err, &appErr) || appErr.Code != "INTERNAL" {
		t.Fatalf("error = %v, want INTERNAL", err)
	}
}
