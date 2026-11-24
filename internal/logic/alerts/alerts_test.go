package alerts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/alerts"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeAlertModel struct {
	model.AlertModel
	items []model.Alert
	total int64
}

func (f fakeAlertModel) List(context.Context, model.AlertFilter) ([]model.Alert, int64, error) {
	return f.items, f.total, nil
}
func (f fakeAlertModel) Get(context.Context, int64) (*model.Alert, error) {
	return nil, model.ErrNotFound
}
func (f fakeAlertModel) Notifications(context.Context, int64) ([]model.AlertNotification, error) {
	return nil, nil
}

func errCode(err error) string {
	var appErr *response.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func TestAlertList(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{AlertModel: fakeAlertModel{
		items: []model.Alert{{ID: 1, Level: "critical", Status: "active", Message: "boom", FirstSeenAt: now, LastSeenAt: now}},
		total: 1,
	}}
	l := alerts.NewAlertListLogic(context.Background(), svcCtx)
	resp, err := l.AlertList(&types.AlertListReq{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Data.Total != 1 || resp.Data.Items[0].Message != "boom" || resp.Data.Items[0].LastSeenAt == "" {
		t.Fatalf("unexpected list: %+v", resp.Data)
	}
}

func TestAlertDetailNotFound(t *testing.T) {
	l := alerts.NewAlertDetailLogic(context.Background(), &svc.ServiceContext{AlertModel: fakeAlertModel{}})
	_, err := l.AlertDetail(&types.AlertDetailReq{Id: "7"})
	if errCode(err) != "NOT_FOUND_ALERT" {
		t.Fatalf("code = %q, want NOT_FOUND_ALERT", errCode(err))
	}
}
