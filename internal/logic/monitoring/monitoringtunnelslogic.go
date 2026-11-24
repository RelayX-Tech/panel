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

type MonitoringTunnelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 隧道链路
func NewMonitoringTunnelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MonitoringTunnelsLogic {
	return &MonitoringTunnelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MonitoringTunnelsLogic) MonitoringTunnels(req *types.TunnelListReq) (resp *types.TunnelListResp, err error) {
	if _, _, _, ok := shared.RangeWindow(req.TimeRange, time.Now().UTC()); !ok {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_RANGE, "range", "unsupported range")
	}

	// 链路条目来自 tunnel 节点；拨测指标未上报时保持 0（不补假数据）。
	nodes, _, err := l.svcCtx.NodeModel.List(l.ctx, model.NodeFilter{Page: 1, PageSize: 100, Type: "tunnel"})
	if err != nil {
		return nil, response.Internal(err)
	}
	links := make([]types.TunnelLink, 0, len(nodes))
	for _, n := range nodes {
		if req.TunnelGroupId != "" && n.GroupID != req.TunnelGroupId {
			continue
		}
		status := "available"
		if n.Status != "online" {
			status = "removed"
		}
		if req.Status != "" && req.Status != status {
			continue
		}
		links = append(links, types.TunnelLink{
			TunnelGroupId: n.GroupID, TunnelGroupName: n.GroupName,
			NodeId: n.ID, NodeName: n.Name, Status: status,
			LastProbeAt: shared.TimeStringOpt(n.LastSeenAt),
		})
	}
	return &types.TunnelListResp{
		Code: errcode.OK,
		Data: types.TunnelListData{
			Links:      links,
			Series:     types.TunnelSeries{Latency: make([]types.TimePoint, 0), Loss: make([]types.TimePoint, 0)},
			PathEvents: make([]types.TunnelPathEvent, 0),
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
