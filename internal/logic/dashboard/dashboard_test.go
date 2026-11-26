package dashboard_test

import (
	"context"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/dashboard"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeNodeModel struct {
	model.NodeModel
	entryOnline, entryTotal, tunnelOnline, tunnelTotal int64
	statusCounts                                       []model.StatusCount
}

func (f fakeNodeModel) CountsByTypeStatus(context.Context, time.Time) (int64, int64, int64, int64, error) {
	return f.entryOnline, f.entryTotal, f.tunnelOnline, f.tunnelTotal, nil
}
func (f fakeNodeModel) PortGroupStatusCounts(context.Context) ([]model.StatusCount, error) {
	return f.statusCounts, nil
}

type fakeAlertModel struct {
	model.AlertModel
	active, info, warning, critical int64
}

func (f fakeAlertModel) Counts(context.Context) (int64, int64, int64, int64, error) {
	return f.active, f.info, f.warning, f.critical, nil
}

type fakeClientModel struct {
	model.ClientModel
	online int64
}

func (f fakeClientModel) OnlineSince(context.Context, time.Time) (int64, error) { return f.online, nil }

type fakeTrafficModel struct {
	model.TrafficModel
	up, down int64
	points   []model.TrafficPoint
}

func (f fakeTrafficModel) Totals(context.Context, time.Time, time.Time) (int64, int64, int, error) {
	return f.up, f.down, 0, nil
}
func (f fakeTrafficModel) Series(context.Context, string, string, string, time.Time, time.Time, time.Duration) ([]model.TrafficPoint, error) {
	return f.points, nil
}

func TestDashboardSummary(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		NodeModel: fakeNodeModel{
			entryOnline: 2, entryTotal: 3, tunnelOnline: 1, tunnelTotal: 2,
			statusCounts: []model.StatusCount{{Status: "active", Count: 4}},
		},
		AlertModel:   fakeAlertModel{active: 5, info: 1, warning: 2, critical: 2},
		ClientModel:  fakeClientModel{online: 7},
		TrafficModel: fakeTrafficModel{up: 600, down: 1200},
	}
	l := dashboard.NewDashboardSummaryLogic(context.Background(), svcCtx)
	resp, err := l.DashboardSummary()
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if resp.Data.Nodes.Entry.Online != 2 || resp.Data.Nodes.Tunnel.Total != 2 {
		t.Fatalf("unexpected node counts: %+v", resp.Data.Nodes)
	}
	if resp.Data.Alerts.Active != 5 || resp.Data.Alerts.ByLevel.Critical != 2 {
		t.Fatalf("unexpected alert counts: %+v", resp.Data.Alerts)
	}
	if resp.Data.ClientsOnline != 7 {
		t.Fatalf("clientsOnline = %d", resp.Data.ClientsOnline)
	}
	if resp.Data.PortGroups.ByStatus[0].Status != "active" {
		t.Fatalf("unexpected port group counts: %+v", resp.Data.PortGroups)
	}
	if resp.Data.Throughput.InBps != 10 || resp.Data.Throughput.OutBps != 20 {
		t.Fatalf("throughput = %+v, want 10/20 (600,1200 bytes over 60s)", resp.Data.Throughput)
	}
	if resp.Data.UpdatedAt == "" {
		t.Fatal("updatedAt is empty")
	}
	if resp.Code != "OK" {
		t.Fatalf("code = %q", resp.Code)
	}
}

func TestDashboardTraffic(t *testing.T) {
	svcCtx := &svc.ServiceContext{TrafficModel: fakeTrafficModel{points: []model.TrafficPoint{
		{TS: time.Now().UTC(), UpBytes: 5, DownBytes: 6},
	}}}
	l := dashboard.NewDashboardTrafficLogic(context.Background(), svcCtx)
	resp, err := l.DashboardTraffic(&types.DashboardTrafficReq{TimeRange: "24h"})
	if err != nil {
		t.Fatalf("traffic: %v", err)
	}
	if resp.Data.TimeRange != "24h" || len(resp.Data.Series.Up) != 1 || resp.Data.Series.Up[0].Value != 5 {
		t.Fatalf("unexpected traffic: %+v", resp.Data)
	}
	if _, err := l.DashboardTraffic(&types.DashboardTrafficReq{TimeRange: "bogus"}); err == nil {
		t.Fatal("invalid range should fail")
	}
}
