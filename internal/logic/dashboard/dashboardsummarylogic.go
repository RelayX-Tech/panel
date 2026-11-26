// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package dashboard

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

const onlineWindow = 5 * time.Minute

type DashboardSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 控制台指标卡与横幅
func NewDashboardSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DashboardSummaryLogic {
	return &DashboardSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DashboardSummaryLogic) DashboardSummary() (resp *types.DashboardSummaryResp, err error) {
	now := time.Now().UTC()

	entryOnline, entryTotal, tunnelOnline, tunnelTotal, err := l.svcCtx.NodeModel.CountsByTypeStatus(l.ctx, now.Add(-onlineWindow))
	if err != nil {
		return nil, response.Internal(err)
	}
	statusCounts, err := l.svcCtx.NodeModel.PortGroupStatusCounts(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	active, info, warning, critical, err := l.svcCtx.AlertModel.Counts(l.ctx)
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

	byStatus := make([]types.ByStatusCount, 0, len(statusCounts))
	for _, s := range statusCounts {
		byStatus = append(byStatus, types.ByStatusCount{Status: s.Status, Count: int(s.Count)})
	}

	data := types.DashboardSummaryData{
		Nodes: types.NodeCounts{
			Entry:  types.CountPair{Online: int(entryOnline), Total: int(entryTotal)},
			Tunnel: types.CountPair{Online: int(tunnelOnline), Total: int(tunnelTotal)},
		},
		PortGroups: types.PortGroupCounts{ByStatus: byStatus},
		Alerts: types.AlertCounts{
			Active:  int(active),
			ByLevel: types.AlertLevelCounts{Info: int(info), Warning: int(warning), Critical: int(critical)},
		},
		ClientsOnline: int(clientsOnline),
		Throughput: types.Throughput{
			InBps:  float64(up) / 60,
			OutBps: float64(down) / 60,
		},
		UpdatedAt: shared.TimeString(now),
	}
	return &types.DashboardSummaryResp{
		Code:      errcode.OK,
		Data:      data,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
