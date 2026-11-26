package nodes_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/nodes"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

// ---- fakes（嵌入接口，仅覆写被测路径使用的方法）----

type nodeModelFunc struct {
	model.NodeModel
	listFn   func(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error)
	getFn    func(context.Context, string) (*model.NodeDetail, error)
	eventsFn func(context.Context, string, model.EventFilter) ([]model.NodeEvent, int64, error)
}

func (f nodeModelFunc) List(ctx context.Context, filter model.NodeFilter) ([]model.NodeSummary, int64, error) {
	if f.listFn != nil {
		return f.listFn(ctx, filter)
	}
	return nil, 0, nil
}

func (f nodeModelFunc) Get(ctx context.Context, id string) (*model.NodeDetail, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return nil, model.ErrNotFound
}

func (f nodeModelFunc) Events(ctx context.Context, id string, filter model.EventFilter) ([]model.NodeEvent, int64, error) {
	if f.eventsFn != nil {
		return f.eventsFn(ctx, id, filter)
	}
	return nil, 0, nil
}

type trafficModelFunc struct {
	model.TrafficModel
	seriesFn func(context.Context, string, string, string, time.Time, time.Time, time.Duration) ([]model.TrafficPoint, error)
}

func (f trafficModelFunc) Series(ctx context.Context, entityType, entityID, metric string, from, to time.Time, step time.Duration) ([]model.TrafficPoint, error) {
	if f.seriesFn != nil {
		return f.seriesFn(ctx, entityType, entityID, metric, from, to, step)
	}
	return nil, nil
}

type upgradeModelFunc struct {
	model.UpgradeModel
	activeTaskFn  func(context.Context) (*model.UpgradeTask, error)
	listTargetsFn func(context.Context) ([]model.UpgradeTarget, error)
}

func (f upgradeModelFunc) ActiveTask(ctx context.Context) (*model.UpgradeTask, error) {
	if f.activeTaskFn != nil {
		return f.activeTaskFn(ctx)
	}
	return nil, nil
}

func (f upgradeModelFunc) ListTargets(ctx context.Context) ([]model.UpgradeTarget, error) {
	if f.listTargetsFn != nil {
		return f.listTargetsFn(ctx)
	}
	return nil, nil
}

func ptrTime(t time.Time) *time.Time { return &t }

// ---- 列表 ----

func TestNodeListAppliesDefaultsAndPaging(t *testing.T) {
	var got model.NodeFilter
	svcCtx := &svc.ServiceContext{NodeModel: nodeModelFunc{
		listFn: func(_ context.Context, f model.NodeFilter) ([]model.NodeSummary, int64, error) {
			got = f
			return []model.NodeSummary{{ID: "n1", Name: "node-1"}}, 1, nil
		},
	}}

	resp, err := nodes.NewNodeListLogic(context.Background(), svcCtx).NodeList(&types.NodeListReq{})
	if err != nil {
		t.Fatalf("NodeList() error = %v", err)
	}
	if got.Page != 1 || got.PageSize != 20 {
		t.Fatalf("filter paging = %d/%d, want 1/20", got.Page, got.PageSize)
	}
	if resp.Data.Page != 1 || resp.Data.PageSize != 20 || resp.Data.Total != 1 {
		t.Fatalf("resp paging = %+v, want page 1 size 20 total 1", resp.Data)
	}
}

func TestNodeListInternalError(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: nodeModelFunc{
		listFn: func(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error) {
			return nil, 0, errors.New("boom")
		},
	}}
	_, err := nodes.NewNodeListLogic(context.Background(), svcCtx).NodeList(&types.NodeListReq{Page: 1, PageSize: 20})
	if errCode(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", errCode(err))
	}
}

func TestNodeListFiltersByUpgradingDisplayState(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelFunc{
			listFn: func(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error) {
				return []model.NodeSummary{
					{ID: "n1", Name: "upgrading-node", LastSeenAt: ptrTime(now)},
					{ID: "n2", Name: "idle-node", LastSeenAt: ptrTime(now)},
				}, 2, nil
			},
		},
		UpgradeModel: upgradeModelFunc{
			activeTaskFn: func(context.Context) (*model.UpgradeTask, error) {
				return &model.UpgradeTask{
					Status: model.UpgradeTaskRunning,
					Nodes:  []model.UpgradeTaskNode{{NodeID: "n1", State: model.NodeUpgradeApplying}},
				}, nil
			},
		},
	}

	resp, err := nodes.NewNodeListLogic(context.Background(), svcCtx).NodeList(&types.NodeListReq{Page: 1, PageSize: 20, Status: "upgrading"})
	if err != nil {
		t.Fatalf("NodeList() error = %v", err)
	}
	if resp.Data.Total != 1 || len(resp.Data.Items) != 1 || resp.Data.Items[0].Id != "n1" {
		t.Fatalf("items = %+v, want only n1", resp.Data.Items)
	}
	if resp.Data.Items[0].DisplayState != "upgrading" {
		t.Fatalf("displayState = %q, want upgrading", resp.Data.Items[0].DisplayState)
	}
}

func TestNodeListFiltersByPendingUpgradeDisplayState(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelFunc{
			listFn: func(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error) {
				return []model.NodeSummary{{ID: "n2", Name: "pending-node"}}, 1, nil
			},
		},
		UpgradeModel: upgradeModelFunc{
			listTargetsFn: func(context.Context) ([]model.UpgradeTarget, error) {
				return []model.UpgradeTarget{{NodeID: "n2", ExpiresAt: now.Add(time.Hour)}}, nil
			},
		},
	}

	resp, err := nodes.NewNodeListLogic(context.Background(), svcCtx).NodeList(&types.NodeListReq{Page: 1, PageSize: 20, Status: "pending_upgrade"})
	if err != nil {
		t.Fatalf("NodeList() error = %v", err)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].DisplayState != "pending_upgrade" {
		t.Fatalf("items = %+v, want n2 pending_upgrade", resp.Data.Items)
	}
}

func TestNodeListIgnoresExpiredTargets(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelFunc{
			listFn: func(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error) {
				return []model.NodeSummary{{ID: "n2"}}, 1, nil
			},
		},
		UpgradeModel: upgradeModelFunc{
			listTargetsFn: func(context.Context) ([]model.UpgradeTarget, error) {
				return []model.UpgradeTarget{{NodeID: "n2", ExpiresAt: now.Add(-time.Hour)}}, nil
			},
		},
	}

	resp, err := nodes.NewNodeListLogic(context.Background(), svcCtx).NodeList(&types.NodeListReq{Page: 1, PageSize: 20, Status: "pending_upgrade"})
	if err != nil {
		t.Fatalf("NodeList() error = %v", err)
	}
	if len(resp.Data.Items) != 0 {
		t.Fatalf("items = %+v, want none (target expired)", resp.Data.Items)
	}
}

// ---- 详情 ----

func TestNodeDetailSuccessMapsFields(t *testing.T) {
	checkedAt := time.Now().UTC()
	detail := &model.NodeDetail{
		NodeSummary: model.NodeSummary{ID: "n1", Name: "node-1", IP: "10.0.0.1"},
		NICs:        []model.NodeNIC{{Name: "eth0", MAC: "aa:bb", MTU: 1500, Up: true, Addresses: []string{"10.0.0.1/24"}}},
		Probe:       model.NodeProbeInfo{Type: "tcp", Success: true, CheckedAt: &checkedAt},
	}
	svcCtx := &svc.ServiceContext{NodeModel: nodeModelFunc{
		getFn: func(context.Context, string) (*model.NodeDetail, error) { return detail, nil },
	}}

	resp, err := nodes.NewNodeDetailLogic(context.Background(), svcCtx).NodeDetail(&types.NodeDetailReq{Id: "n1"})
	if err != nil {
		t.Fatalf("NodeDetail() error = %v", err)
	}
	if resp.Data.Id != "n1" || resp.Data.Ip != "10.0.0.1" {
		t.Fatalf("detail = %+v", resp.Data)
	}
	if len(resp.Data.Nics) != 1 || resp.Data.Nics[0].Name != "eth0" || !resp.Data.Nics[0].Up {
		t.Fatalf("nics = %+v", resp.Data.Nics)
	}
	if resp.Data.Probe.CheckedAt == "" {
		t.Fatal("probe.checkedAt must be populated")
	}
}

func TestNodeDetailInternalError(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: nodeModelFunc{
		getFn: func(context.Context, string) (*model.NodeDetail, error) { return nil, errors.New("db down") },
	}}
	_, err := nodes.NewNodeDetailLogic(context.Background(), svcCtx).NodeDetail(&types.NodeDetailReq{Id: "n1"})
	if errCode(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", errCode(err))
	}
}

// ---- 事件 ----

func TestNodeEventsRejectsInvalidTime(t *testing.T) {
	l := nodes.NewNodeEventsLogic(context.Background(), &svc.ServiceContext{NodeModel: nodeModelFunc{}})
	if _, err := l.NodeEvents(&types.NodeEventsReq{Id: "n1", From: "not-a-time"}); errCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("from code = %q, want VALIDATION_INVALID_FIELD", errCode(err))
	}
	if _, err := l.NodeEvents(&types.NodeEventsReq{Id: "n1", To: "not-a-time"}); errCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("to code = %q, want VALIDATION_INVALID_FIELD", errCode(err))
	}
}

func TestNodeEventsMapsMetadata(t *testing.T) {
	occurred := time.Now().UTC()
	var gotFilter model.EventFilter
	svcCtx := &svc.ServiceContext{NodeModel: nodeModelFunc{
		eventsFn: func(_ context.Context, _ string, f model.EventFilter) ([]model.NodeEvent, int64, error) {
			gotFilter = f
			return []model.NodeEvent{{
				ID: 1, NodeID: "n1", Type: "restart", Level: "warning", Message: "restarted",
				Metadata: []byte(`{"reason":"oom"}`), OccurredAt: occurred,
			}}, 1, nil
		},
	}}

	resp, err := nodes.NewNodeEventsLogic(context.Background(), svcCtx).NodeEvents(&types.NodeEventsReq{
		Id: "n1", Page: 2, PageSize: 10, EventType: "restart",
		From: "2026-11-25T00:00:00Z", To: "2026-11-26T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("NodeEvents() error = %v", err)
	}
	if gotFilter.From == nil || gotFilter.To == nil || gotFilter.Type != "restart" {
		t.Fatalf("filter = %+v, want from/to/type populated", gotFilter)
	}
	if resp.Data.Total != 1 || resp.Data.Page != 2 || resp.Data.PageSize != 10 {
		t.Fatalf("resp = %+v", resp.Data)
	}
	meta, ok := resp.Data.Items[0].Metadata.(map[string]any)
	if !ok || meta["reason"] != "oom" {
		t.Fatalf("metadata = %#v, want reason=oom", resp.Data.Items[0].Metadata)
	}
}

func TestNodeEventsInternalError(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: nodeModelFunc{
		eventsFn: func(context.Context, string, model.EventFilter) ([]model.NodeEvent, int64, error) {
			return nil, 0, errors.New("boom")
		},
	}}
	_, err := nodes.NewNodeEventsLogic(context.Background(), svcCtx).NodeEvents(&types.NodeEventsReq{Id: "n1"})
	if errCode(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", errCode(err))
	}
}

// ---- 指标 ----

func TestNodeMetricsRejectsUnsupportedMetric(t *testing.T) {
	l := nodes.NewNodeMetricsLogic(context.Background(), &svc.ServiceContext{NodeModel: nodeModelFunc{}})
	_, err := l.NodeMetrics(&types.NodeMetricsReq{Id: "n1", Metrics: "bogus"})
	if errCode(err) != "VALIDATION_INVALID_FIELD" {
		t.Fatalf("code = %q, want VALIDATION_INVALID_FIELD", errCode(err))
	}
}

func TestNodeMetricsRejectsInvalidRange(t *testing.T) {
	l := nodes.NewNodeMetricsLogic(context.Background(), &svc.ServiceContext{NodeModel: nodeModelFunc{}})
	_, err := l.NodeMetrics(&types.NodeMetricsReq{Id: "n1", TimeRange: "5y"})
	if errCode(err) != "VALIDATION_INVALID_RANGE" {
		t.Fatalf("code = %q, want VALIDATION_INVALID_RANGE", errCode(err))
	}
}

func TestNodeMetricsNodeNotFound(t *testing.T) {
	l := nodes.NewNodeMetricsLogic(context.Background(), &svc.ServiceContext{NodeModel: nodeModelFunc{}})
	_, err := l.NodeMetrics(&types.NodeMetricsReq{Id: "missing", TimeRange: "24h"})
	if errCode(err) != "NODE_NOT_FOUND" {
		t.Fatalf("code = %q, want NODE_NOT_FOUND", errCode(err))
	}
}

func TestNodeMetricsNetSeriesAggregatesUpDown(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelFunc{
			getFn: func(context.Context, string) (*model.NodeDetail, error) {
				return &model.NodeDetail{NodeSummary: model.NodeSummary{ID: "n1"}}, nil
			},
		},
		TrafficModel: trafficModelFunc{
			seriesFn: func(context.Context, string, string, string, time.Time, time.Time, time.Duration) ([]model.TrafficPoint, error) {
				return []model.TrafficPoint{{TS: now, UpBytes: 100, DownBytes: 200}}, nil
			},
		},
	}

	resp, err := nodes.NewNodeMetricsLogic(context.Background(), svcCtx).NodeMetrics(&types.NodeMetricsReq{Id: "n1", TimeRange: "24h", Metrics: "net,cpu"})
	if err != nil {
		t.Fatalf("NodeMetrics() error = %v", err)
	}
	if len(resp.Data.Series) != 2 {
		t.Fatalf("series = %+v, want 2", resp.Data.Series)
	}
	if resp.Data.Series[0].Metric != "net" || len(resp.Data.Series[0].Points) != 1 || resp.Data.Series[0].Points[0].Value != 300 {
		t.Fatalf("net series = %+v, want single 300 point", resp.Data.Series[0])
	}
}

func TestNodeMetricsInternalErrorFromSeries(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		NodeModel: nodeModelFunc{
			getFn: func(context.Context, string) (*model.NodeDetail, error) {
				return &model.NodeDetail{NodeSummary: model.NodeSummary{ID: "n1"}}, nil
			},
		},
		TrafficModel: trafficModelFunc{
			seriesFn: func(context.Context, string, string, string, time.Time, time.Time, time.Duration) ([]model.TrafficPoint, error) {
				return nil, errors.New("boom")
			},
		},
	}
	_, err := nodes.NewNodeMetricsLogic(context.Background(), svcCtx).NodeMetrics(&types.NodeMetricsReq{Id: "n1", TimeRange: "24h", Metrics: "net"})
	if errCode(err) != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", errCode(err))
	}
}

// ---- 拨测 ----

func TestNodeProbeValidation(t *testing.T) {
	l := nodes.NewNodeProbeLogic(context.Background(), &svc.ServiceContext{NodeModel: nodeModelFunc{}})
	cases := []struct {
		name string
		req  *types.NodeProbeReq
		code string
	}{
		{"bad-type", &types.NodeProbeReq{Id: "n1", ProbeType: "icmp", Target: "x"}, "VALIDATION_INVALID_FIELD"},
		{"missing-target", &types.NodeProbeReq{Id: "n1", ProbeType: "tcp"}, "VALIDATION_MISSING_FIELD"},
		{"timeout-too-large", &types.NodeProbeReq{Id: "n1", ProbeType: "tcp", Target: "x", TimeoutMs: 60000}, "VALIDATION_INVALID_FIELD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.NodeProbe(tc.req); errCode(err) != tc.code {
				t.Fatalf("code = %q, want %s", errCode(err), tc.code)
			}
		})
	}
}

func TestNodeProbeNodeNotFound(t *testing.T) {
	l := nodes.NewNodeProbeLogic(context.Background(), &svc.ServiceContext{NodeModel: nodeModelFunc{}})
	_, err := l.NodeProbe(&types.NodeProbeReq{Id: "missing", ProbeType: "tcp", Target: "10.0.0.1:443"})
	if errCode(err) != "NODE_NOT_FOUND" {
		t.Fatalf("code = %q, want NODE_NOT_FOUND", errCode(err))
	}
}

func TestNodeProbeAgentNotConfigured(t *testing.T) {
	svcCtx := &svc.ServiceContext{NodeModel: nodeModelFunc{
		getFn: func(context.Context, string) (*model.NodeDetail, error) {
			return &model.NodeDetail{NodeSummary: model.NodeSummary{ID: "n1"}}, nil
		},
	}}
	_, err := nodes.NewNodeProbeLogic(context.Background(), svcCtx).NodeProbe(&types.NodeProbeReq{Id: "n1", ProbeType: "tcp", Target: "10.0.0.1:443"})
	if errCode(err) != "UPSTREAM_AGENT_UNAVAILABLE" {
		t.Fatalf("code = %q, want UPSTREAM_AGENT_UNAVAILABLE", errCode(err))
	}
}
