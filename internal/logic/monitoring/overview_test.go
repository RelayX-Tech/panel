package monitoring_test

import (
	"context"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/monitoring"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeNodeModel struct {
	model.NodeModel
}

func (f fakeNodeModel) CountsByTypeStatus(context.Context, time.Time) (int64, int64, int64, int64, error) {
	return 1, 2, 0, 0, nil
}
func (f fakeNodeModel) XrayAbnormal(context.Context) (int64, error) { return 0, nil }
func (f fakeNodeModel) GroupNodes(context.Context) ([]model.NodeHealthGroup, error) {
	return nil, nil
}

type fakeAlertModel struct{ model.AlertModel }

func (f fakeAlertModel) Counts(context.Context) (int64, int64, int64, int64, error) {
	return 0, 0, 0, 0, nil
}
func (f fakeAlertModel) Trend(context.Context, time.Time, time.Time, time.Duration) ([]model.TrendPoint, []model.TrendPoint, []model.TrendPoint, error) {
	return nil, nil, nil, nil
}
func (f fakeAlertModel) Recent(context.Context, int) ([]model.Alert, error) { return nil, nil }

type fakeClientModel struct{ model.ClientModel }

func (f fakeClientModel) OnlineSince(context.Context, time.Time) (int64, error) { return 0, nil }

type fakeTrafficModel struct{ model.TrafficModel }

func (f fakeTrafficModel) Series(context.Context, string, string, string, time.Time, time.Time, time.Duration) ([]model.TrafficPoint, error) {
	return nil, nil
}
func (f fakeTrafficModel) Totals(context.Context, time.Time, time.Time) (int64, int64, int, error) {
	return 0, 0, 0, nil
}
func (f fakeTrafficModel) Rows(context.Context, string, time.Time, time.Time, int, int, string) ([]model.TrafficRow, int64, error) {
	return nil, 0, nil
}

func TestMonitoringOverviewEmpty(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		NodeModel: fakeNodeModel{}, AlertModel: fakeAlertModel{},
		ClientModel: fakeClientModel{}, TrafficModel: fakeTrafficModel{},
	}
	l := monitoring.NewMonitoringOverviewLogic(context.Background(), svcCtx)
	resp, err := l.MonitoringOverview(&types.MonitoringOverviewReq{TimeRange: "24h"})
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if resp.Data.Cards.OnlineNodes != 1 || resp.Data.Cards.OnlineClients != 0 {
		t.Fatalf("unexpected cards: %+v", resp.Data.Cards)
	}
	if resp.Data.NodeHealth == nil || resp.Data.RecentAlerts == nil || resp.Data.TopPortGroups == nil {
		t.Fatal("empty slices must be non-nil (no fake data, but stable JSON arrays)")
	}
}
