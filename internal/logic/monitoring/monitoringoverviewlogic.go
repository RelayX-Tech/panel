// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package monitoring

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

const (
	onlineWindow = 5 * time.Minute
	trendTopN    = 5
)

type MonitoringOverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 监控总览
func NewMonitoringOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MonitoringOverviewLogic {
	return &MonitoringOverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MonitoringOverviewLogic) MonitoringOverview(req *types.MonitoringOverviewReq) (resp *types.MonitoringOverviewResp, err error) {
	now := time.Now().UTC()
	from, to, step, ok := shared.RangeWindow(req.TimeRange, now)
	if !ok {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_RANGE, "range", "unsupported range")
	}

	entryOnline, _, tunnelOnline, _, err := l.svcCtx.NodeModel.CountsByTypeStatus(l.ctx, now.Add(-onlineWindow))
	if err != nil {
		return nil, response.Internal(err)
	}
	xrayAbnormal, err := l.svcCtx.NodeModel.XrayAbnormal(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	active, _, _, _, err := l.svcCtx.AlertModel.Counts(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	clientsOnline, err := l.svcCtx.ClientModel.OnlineSince(l.ctx, now.Add(-onlineWindow))
	if err != nil {
		return nil, response.Internal(err)
	}
	up, down, _, err := l.svcCtx.TrafficModel.Totals(l.ctx, now.Add(-time.Minute), now)
	if err != nil {
		return nil, response.Internal(err)
	}
	trafficPoints, err := l.svcCtx.TrafficModel.Series(l.ctx, "global", "", "net", from, to, step)
	if err != nil {
		return nil, response.Internal(err)
	}
	upSeries := make([]types.TimePoint, 0, len(trafficPoints))
	downSeries := make([]types.TimePoint, 0, len(trafficPoints))
	for _, p := range trafficPoints {
		ts := shared.TimeString(p.TS)
		upSeries = append(upSeries, types.TimePoint{Ts: ts, Value: float64(p.UpBytes)})
		downSeries = append(downSeries, types.TimePoint{Ts: ts, Value: float64(p.DownBytes)})
	}

	infoTrend, warningTrend, criticalTrend, err := l.svcCtx.AlertModel.Trend(l.ctx, from, to, step)
	if err != nil {
		return nil, response.Internal(err)
	}
	groups, err := l.svcCtx.NodeModel.GroupNodes(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	topRows, _, err := l.svcCtx.TrafficModel.Rows(l.ctx, "port_group", from, to, 1, trendTopN, "")
	if err != nil {
		return nil, response.Internal(err)
	}
	recent, err := l.svcCtx.AlertModel.Recent(l.ctx, trendTopN)
	if err != nil {
		return nil, response.Internal(err)
	}

	return &types.MonitoringOverviewResp{
		Code: errcode.OK,
		Data: types.MonitoringOverviewData{
			Cards: types.OverviewCards{
				ActiveAlerts:  int(active),
				OnlineNodes:   int(entryOnline + tunnelOnline),
				OnlineClients: int(clientsOnline),
				InBps:         float64(up) / 60,
				OutBps:        float64(down) / 60,
				XrayAbnormal:  int(xrayAbnormal),
			},
			Traffic: types.TrafficSeries{Up: upSeries, Down: downSeries},
			AlertTrend: types.AlertTrend{
				Info: toPoints(infoTrend), Warning: toPoints(warningTrend), Critical: toPoints(criticalTrend),
			},
			NodeHealth:    toHealthGroups(groups),
			TopPortGroups: toTopPortGroups(topRows),
			RecentAlerts:  toRecentAlerts(recent),
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}

func toPoints(in []model.TrendPoint) []types.TimePoint {
	out := make([]types.TimePoint, 0, len(in))
	for _, p := range in {
		out = append(out, types.TimePoint{Ts: shared.TimeString(p.TS), Value: p.Value})
	}
	return out
}

func toHealthGroups(in []model.NodeHealthGroup) []types.NodeHealthGroup {
	out := make([]types.NodeHealthGroup, 0, len(in))
	for _, g := range in {
		nodes := make([]types.NodeHealthNode, 0, len(g.Nodes))
		for _, n := range g.Nodes {
			nodes = append(nodes, types.NodeHealthNode{
				NodeId: n.NodeID, NodeName: n.NodeName, Ip: n.IP, Status: n.Status,
				CpuPercent: n.CPUPercent, MemPercent: n.MemPercent, XrayState: n.XrayState,
			})
		}
		out = append(out, types.NodeHealthGroup{
			GroupType: g.GroupType, GroupId: g.GroupID, GroupName: g.GroupName, Nodes: nodes,
		})
	}
	return out
}

func toTopPortGroups(in []model.TrafficRow) []types.TopPortGroup {
	out := make([]types.TopPortGroup, 0, len(in))
	for _, r := range in {
		out = append(out, types.TopPortGroup{
			PortGroupId: r.ObjectID, PortGroupName: r.ObjectName, TotalBytes: r.TotalBytes, Connections: r.Connections,
		})
	}
	return out
}

func toRecentAlerts(in []model.Alert) []types.RecentAlert {
	out := make([]types.RecentAlert, 0, len(in))
	for _, a := range in {
		out = append(out, types.RecentAlert{
			Id: a.ID, Level: a.Level, AlertType: a.Type, ObjectName: a.ObjectName,
			Message: a.Message, LastSeenAt: shared.TimeString(a.LastSeenAt),
		})
	}
	return out
}
