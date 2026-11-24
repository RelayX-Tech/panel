package clients_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/logic/clients"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeClientModel struct {
	model.ClientModel
	rows  []model.ClientRow
	total int64
}

func (f fakeClientModel) List(context.Context, model.ClientFilter) ([]model.ClientRow, int64, error) {
	return f.rows, f.total, nil
}

func TestClientList(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{ClientModel: fakeClientModel{
		rows:  []model.ClientRow{{ClientID: "c1", Email: "a@example.com", PortID: "p1", UpBytes: 1, DownBytes: 2, LastOnlineAt: &now}},
		total: 1,
	}}
	ctx := auth.ContextWithSession(context.Background(), &model.SessionWithUser{
		Session: model.Session{ID: "s1"},
		User:    model.User{ID: "u1", Account: "admin"},
	})
	l := clients.NewClientListLogic(ctx, svcCtx)
	resp, err := l.ClientList(&types.ClientListReq{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Data.Total != 1 || resp.Data.Items[0].Email != "a@example.com" || resp.Data.Items[0].LastOnlineAt == "" {
		t.Fatalf("unexpected list: %+v", resp.Data)
	}
}

type roleModel struct{ model.AuthUserModel }

func (roleModel) RoleOf(context.Context, string) (string, error) { return "viewer", nil }

func TestClientListRequiresAdmin(t *testing.T) {
	svcCtx := &svc.ServiceContext{UserModel: roleModel{}, ClientModel: fakeClientModel{}}
	ctx := auth.ContextWithSession(context.Background(), &model.SessionWithUser{
		Session: model.Session{ID: "s1"},
		User:    model.User{ID: "u1", Account: "viewer"},
	})
	_, err := clients.NewClientListLogic(ctx, svcCtx).ClientList(&types.ClientListReq{Page: 1, PageSize: 20})
	var appErr *response.AppError
	if !errors.As(err, &appErr) || appErr.Code != "AUTH_FORBIDDEN" {
		t.Fatalf("err = %v, want AUTH_FORBIDDEN", err)
	}
}
