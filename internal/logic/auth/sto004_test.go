package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/relayx-tech/panel/internal/auth"
	logicauth "github.com/relayx-tech/panel/internal/logic/auth"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeUserModel struct {
	model.AuthUserModel
	user    *model.User
	updated string
}

func (f *fakeUserModel) FindOne(context.Context, string) (*model.User, error) { return f.user, nil }
func (f *fakeUserModel) UpdateDisplayName(_ context.Context, _ sqlx.Session, id, name string) error {
	f.updated = name
	return nil
}

type fakeSessionModel struct {
	model.AuthSessionModel
	sessions []model.Session
	revoked  []string
}

func (f *fakeSessionModel) ListByUserID(context.Context, string) ([]model.Session, error) {
	return f.sessions, nil
}
func (f *fakeSessionModel) RevokeByID(_ context.Context, _ sqlx.Session, id string) error {
	f.revoked = append(f.revoked, id)
	return nil
}
func (f *fakeSessionModel) FindByID(_ context.Context, id string) (*model.Session, error) {
	for i := range f.sessions {
		if f.sessions[i].ID == id {
			return &f.sessions[i], nil
		}
	}
	return nil, model.ErrNotFound
}

type fakeAuditModel struct {
	model.AuditLogModel
	logged []model.AuditLog
}

func (f *fakeAuditModel) Insert(_ context.Context, l model.AuditLog) error {
	f.logged = append(f.logged, l)
	return nil
}

func errCode(err error) string {
	var appErr *response.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func sessionCtx() context.Context {
	return auth.ContextWithSession(context.Background(), &model.SessionWithUser{
		Session: model.Session{ID: "s1"},
		User:    model.User{ID: "u1", Account: "admin", DisplayName: "Admin"},
	})
}

func TestProfileUpdate(t *testing.T) {
	users := &fakeUserModel{user: &model.User{ID: "u1", Account: "admin", DisplayName: "Admin"}}
	audit := &fakeAuditModel{}
	l := logicauth.NewProfileUpdateLogic(sessionCtx(), &svc.ServiceContext{UserModel: users, AuditLogModel: audit})
	resp, err := l.ProfileUpdate(&types.ProfileUpdateReq{DisplayName: "Ops"})
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if users.updated != "Ops" || resp.Data.DisplayName != "Ops" {
		t.Fatalf("not updated: %q %+v", users.updated, resp.Data)
	}
	if len(audit.logged) != 1 || audit.logged[0].Action != "profile.update" {
		t.Fatalf("audit not written: %+v", audit.logged)
	}

	if _, err := l.ProfileUpdate(&types.ProfileUpdateReq{DisplayName: "  "}); errCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("blank displayName code = %q", errCode(err))
	}
}

func TestSessionListMarksCurrent(t *testing.T) {
	now := time.Now()
	sessions := &fakeSessionModel{sessions: []model.Session{
		{ID: "s1", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(time.Hour), ClientIP: "1.2.3.4", UserAgent: "ua"},
		{ID: "s2", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(time.Hour)},
	}}
	l := logicauth.NewSessionListLogic(sessionCtx(), &svc.ServiceContext{SessionModel: sessions})
	resp, err := l.SessionList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(resp.Data.Items) != 2 || !resp.Data.Items[0].Current || resp.Data.Items[1].Current {
		t.Fatalf("current flag wrong: %+v", resp.Data.Items)
	}
}

func TestSessionRevokeSelfForbidden(t *testing.T) {
	sessions := &fakeSessionModel{}
	audit := &fakeAuditModel{}
	l := logicauth.NewSessionRevokeLogic(sessionCtx(), &svc.ServiceContext{SessionModel: sessions, AuditLogModel: audit})
	_, err := l.SessionRevoke(&types.SessionRevokeReq{Id: "s1"})
	if errCode(err) != "FORBIDDEN_SELF_SESSION_REVOKE" {
		t.Fatalf("code = %q", errCode(err))
	}
	if len(sessions.revoked) != 0 {
		t.Fatal("self session must not be revoked")
	}
}

func TestSessionRevokeOther(t *testing.T) {
	now := time.Now()
	sessions := &fakeSessionModel{sessions: []model.Session{{ID: "s2", UserID: "u1", CreatedAt: now, LastSeenAt: now, ExpiresAt: now, AbsoluteExpiresAt: now}}}
	audit := &fakeAuditModel{}
	l := logicauth.NewSessionRevokeLogic(sessionCtx(), &svc.ServiceContext{SessionModel: sessions, AuditLogModel: audit})
	if _, err := l.SessionRevoke(&types.SessionRevokeReq{Id: "s2"}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(sessions.revoked) != 1 || sessions.revoked[0] != "s2" {
		t.Fatalf("not revoked: %+v", sessions.revoked)
	}
	if len(audit.logged) != 1 || audit.logged[0].Action != "session.revoke" {
		t.Fatalf("audit: %+v", audit.logged)
	}
}
