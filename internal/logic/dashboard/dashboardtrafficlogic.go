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

type DashboardTrafficLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 控制台流量趋势
func NewDashboardTrafficLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DashboardTrafficLogic {
	return &DashboardTrafficLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DashboardTrafficLogic) DashboardTraffic(req *types.DashboardTrafficReq) (resp *types.DashboardTrafficResp, err error) {
	now := time.Now().UTC()
	from, to, step, ok := shared.RangeWindow(req.TimeRange, now)
	if !ok {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_RANGE, "range", "unsupported range")
	}
	points, err := l.svcCtx.TrafficModel.Series(l.ctx, "global", "", "net", from, to, step)
	if err != nil {
		return nil, response.Internal(err)
	}
	up := make([]types.TimePoint, 0, len(points))
	down := make([]types.TimePoint, 0, len(points))
	for _, p := range points {
		ts := shared.TimeString(p.TS)
		up = append(up, types.TimePoint{Ts: ts, Value: float64(p.UpBytes)})
		down = append(down, types.TimePoint{Ts: ts, Value: float64(p.DownBytes)})
	}
	return &types.DashboardTrafficResp{
		Code: errcode.OK,
		Data: types.DashboardTrafficData{
			TimeRange: shared.RangeOrDefault(req.TimeRange),
			Unit:      "bytes",
			Series:    types.TrafficSeries{Up: up, Down: down},
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
