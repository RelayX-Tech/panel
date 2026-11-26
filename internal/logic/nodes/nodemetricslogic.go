// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package nodes

import (
	"context"
	"errors"
	"strings"
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

// 指标白名单：design.md §3.5 API-06。
var metricUnits = map[string]string{
	"cpu":         "percent",
	"mem":         "percent",
	"disk":        "percent",
	"diskIo":      "bytes/s",
	"net":         "bytes/s",
	"load":        "",
	"connections": "",
}

type NodeMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 节点资源与内核序列
func NewNodeMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeMetricsLogic {
	return &NodeMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeMetricsLogic) NodeMetrics(req *types.NodeMetricsReq) (resp *types.NodeMetricsResp, err error) {
	var requested []string
	if strings.TrimSpace(req.Metrics) == "" {
		requested = []string{"cpu", "mem", "disk", "diskIo", "net", "load", "connections"}
	} else {
		requested = normalizeIDList(req.Metrics)
	}
	for _, m := range requested {
		if _, ok := metricUnits[m]; !ok {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "metrics", "unsupported metric")
		}
	}

	now := time.Now().UTC()
	from, to, step, ok := shared.RangeWindow(req.TimeRange, now)
	if !ok {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_RANGE, "range", "unsupported range")
	}
	if _, err := l.svcCtx.NodeModel.Get(l.ctx, req.Id); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NODE_NOT_FOUND, "node not found")
		}
		return nil, response.Internal(err)
	}

	series := make([]types.MetricSeries, 0, len(requested))
	for _, metric := range requested {
		points := make([]types.TimePoint, 0)
		if metric == "net" {
			raw, err := l.svcCtx.TrafficModel.Series(l.ctx, "node", req.Id, "net", from, to, step)
			if err != nil {
				return nil, response.Internal(err)
			}
			for _, p := range raw {
				points = append(points, types.TimePoint{Ts: shared.TimeString(p.TS), Value: float64(p.UpBytes + p.DownBytes)})
			}
		}
		series = append(series, types.MetricSeries{Metric: metric, Unit: metricUnits[metric], Points: points})
	}
	return &types.NodeMetricsResp{
		Code:      errcode.OK,
		Data:      types.NodeMetricsData{Series: series},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
