package traffic_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/traffic"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeTrafficModel struct {
	model.TrafficModel
	rows  []model.TrafficRow
	total int64
}

func (f fakeTrafficModel) Rows(context.Context, string, time.Time, time.Time, int, int, string) ([]model.TrafficRow, int64, error) {
	return f.rows, f.total, nil
}

func errCode(err error) string {
	var appErr *response.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func TestTrafficList(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{TrafficModel: fakeTrafficModel{
		rows:  []model.TrafficRow{{ObjectType: "port_group", ObjectID: "pg1", ObjectName: "pg1", UpBytes: 10, DownBytes: 20, TotalBytes: 30, LastUpdatedAt: now}},
		total: 1,
	}}
	l := traffic.NewTrafficListLogic(context.Background(), svcCtx)
	resp, err := l.TrafficList(&types.TrafficListReq{GroupBy: "portGroup", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Data.Total != 1 || resp.Data.Items[0].TotalBytes != 30 || resp.Data.Unit != "bytes" {
		t.Fatalf("unexpected list: %+v", resp.Data)
	}

	if _, err := l.TrafficList(&types.TrafficListReq{GroupBy: "bogus"}); errCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("invalid groupBy code = %q", errCode(err))
	}
}
