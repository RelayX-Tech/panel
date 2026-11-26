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

type tunnelNodes struct{ model.NodeModel }

func (tunnelNodes) List(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error) {
	now := time.Now().UTC()
	return []model.NodeSummary{
		{ID: "t1", Name: "tunnel-1", Type: "tunnel", GroupID: "tg1", GroupName: "Tokyo", Status: "online", LastSeenAt: &now},
		{ID: "t2", Name: "tunnel-2", Type: "tunnel", GroupID: "tg1", GroupName: "Tokyo", Status: "offline"},
	}, 2, nil
}

func TestMonitoringTunnelsFromNodes(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: tunnelNodes{}}
	resp, err := monitoring.NewMonitoringTunnelsLogic(context.Background(), svcCtx).MonitoringTunnels(&types.TunnelListReq{TimeRange: "1h"})
	if err != nil {
		t.Fatalf("tunnels: %v", err)
	}
	if len(resp.Data.Links) != 2 {
		t.Fatalf("links = %+v", resp.Data.Links)
	}
	byID := map[string]types.TunnelLink{}
	for _, l := range resp.Data.Links {
		byID[l.NodeId] = l
	}
	if byID["t1"].Status != "available" || byID["t2"].Status != "removed" {
		t.Fatalf("unexpected statuses: %+v", byID)
	}
	if byID["t1"].TunnelGroupName != "Tokyo" {
		t.Fatalf("group name missing: %+v", byID["t1"])
	}
}
