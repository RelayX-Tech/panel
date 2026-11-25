package audit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/audit"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeAuditModel struct {
	model.AuditLogModel
	items     []model.AuditLog
	total     int64
	item      *model.AuditLog
	listErr   error
	getErr    error
	gotFilter model.AuditFilter
}

func (f *fakeAuditModel) List(_ context.Context, filter model.AuditFilter) ([]model.AuditLog, int64, error) {
	f.gotFilter = filter
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.items, f.total, nil
}

func (f *fakeAuditModel) Get(_ context.Context, _ int64) (*model.AuditLog, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.item, nil
}

func appErrCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not *response.AppError", err)
	}
	return appErr.Code
}

func TestAuditLogListMapsFieldsAndForwardsFilter(t *testing.T) {
	createdAt := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
	fake := &fakeAuditModel{
		items: []model.AuditLog{{
			ID: 7, Actor: "admin", Action: "upgrade.panel.start", ObjectType: "upgrade_task",
			ObjectID: "task-1", Result: "success", RequestID: "req-abc", IP: "203.0.113.5", CreatedAt: createdAt,
		}},
		total: 1,
	}
	l := audit.NewAuditLogListLogic(context.Background(), &svc.ServiceContext{AuditLogModel: fake})

	resp, err := l.AuditLogList(&types.AuditLogListReq{
		Actor: "admin", From: "2026-11-23T00:00:00Z", Page: 2, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Data.Total != 1 || resp.Data.Page != 2 || resp.Data.PageSize != 10 {
		t.Fatalf("unexpected paging: %+v", resp.Data)
	}
	if len(resp.Data.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Data.Items))
	}
	item := resp.Data.Items[0]
	if item.Id != 7 || item.Actor != "admin" || item.Action != "upgrade.panel.start" ||
		item.ObjectType != "upgrade_task" || item.ObjectId != "task-1" ||
		item.Result != "success" || item.RequestId != "req-abc" || item.Ip != "203.0.113.5" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.Ts != "2026-11-23T14:00:00.000Z" {
		t.Fatalf("ts = %q, want RFC3339 millis", item.Ts)
	}
	if fake.gotFilter.Actor != "admin" || fake.gotFilter.Page != 2 || fake.gotFilter.PageSize != 10 {
		t.Fatalf("filter not forwarded: %+v", fake.gotFilter)
	}
	if fake.gotFilter.From == nil || !fake.gotFilter.From.Equal(time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from not parsed: %+v", fake.gotFilter.From)
	}
}

func TestAuditLogListRejectsInvalidTimeRange(t *testing.T) {
	l := audit.NewAuditLogListLogic(context.Background(), &svc.ServiceContext{AuditLogModel: &fakeAuditModel{}})

	for _, req := range []*types.AuditLogListReq{
		{Page: 1, PageSize: 20, From: "not-a-time"},
		{Page: 1, PageSize: 20, To: "not-a-time"},
	} {
		_, err := l.AuditLogList(req)
		if code := appErrCode(t, err); code != "VALIDATION_INVALID_FIELD" {
			t.Fatalf("code = %q, want VALIDATION_INVALID_FIELD", code)
		}
	}
}

func TestAuditLogListWrapsModelError(t *testing.T) {
	l := audit.NewAuditLogListLogic(context.Background(), &svc.ServiceContext{
		AuditLogModel: &fakeAuditModel{listErr: errors.New("db down")},
	})
	_, err := l.AuditLogList(&types.AuditLogListReq{Page: 1, PageSize: 20})
	if code := appErrCode(t, err); code != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", code)
	}
}

func TestAuditLogDetailReturnsItem(t *testing.T) {
	createdAt := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
	fake := &fakeAuditModel{item: &model.AuditLog{
		ID: 9, Actor: "admin", Action: "upgrade.panel.start", ObjectType: "upgrade_task",
		ObjectID: "task-9", Result: "success", RequestID: "req-9", IP: "198.51.100.9", CreatedAt: createdAt,
	}}
	l := audit.NewAuditLogDetailLogic(context.Background(), &svc.ServiceContext{AuditLogModel: fake})

	resp, err := l.AuditLogDetail(&types.AuditLogDetailReq{Id: "9"})
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if resp.Data.Item.Id != 9 || resp.Data.Item.RequestId != "req-9" || resp.Data.Item.Ts == "" {
		t.Fatalf("unexpected detail: %+v", resp.Data.Item)
	}
}

func TestAuditLogDetailNotFound(t *testing.T) {
	cases := []struct {
		name string
		id   string
		fake *fakeAuditModel
	}{
		{name: "non numeric id", id: "abc", fake: &fakeAuditModel{}},
		{name: "zero id", id: "0", fake: &fakeAuditModel{}},
		{name: "negative id", id: "-3", fake: &fakeAuditModel{}},
		{name: "missing row", id: "7", fake: &fakeAuditModel{getErr: model.ErrNotFound}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := audit.NewAuditLogDetailLogic(context.Background(), &svc.ServiceContext{AuditLogModel: tc.fake})
			_, err := l.AuditLogDetail(&types.AuditLogDetailReq{Id: tc.id})
			if code := appErrCode(t, err); code != "NOT_FOUND_AUDIT_LOG" {
				t.Fatalf("code = %q, want NOT_FOUND_AUDIT_LOG", code)
			}
		})
	}
}

func TestAuditLogDetailWrapsModelError(t *testing.T) {
	l := audit.NewAuditLogDetailLogic(context.Background(), &svc.ServiceContext{
		AuditLogModel: &fakeAuditModel{getErr: errors.New("db down")},
	})
	_, err := l.AuditLogDetail(&types.AuditLogDetailReq{Id: "7"})
	if code := appErrCode(t, err); code != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", code)
	}
}
