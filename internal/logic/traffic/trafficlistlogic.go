// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package traffic

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

var groupByEntity = map[string]string{
	"portGroup": "port_group",
	"node":      "node",
	"port":      "port",
	"client":    "client",
}

type TrafficListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 流量分组表
func NewTrafficListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TrafficListLogic {
	return &TrafficListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TrafficListLogic) TrafficList(req *types.TrafficListReq) (resp *types.TrafficListResp, err error) {
	entity, ok := groupByEntity[req.GroupBy]
	if !ok {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "groupBy", "unsupported groupBy")
	}
	now := time.Now().UTC()
	from, to := now.Add(-24*time.Hour), now
	if req.From != "" {
		at, perr := time.Parse(time.RFC3339, req.From)
		if perr != nil {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "from", "invalid time")
		}
		from = at
	}
	if req.To != "" {
		at, perr := time.Parse(time.RFC3339, req.To)
		if perr != nil {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "to", "invalid time")
		}
		to = at
	}

	rows, total, err := l.svcCtx.TrafficModel.Rows(l.ctx, entity, from, to, req.Page, req.PageSize, req.Sort)
	if err != nil {
		return nil, response.Internal(err)
	}
	items := make([]types.TrafficRow, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.TrafficRow{
			ObjectType: r.ObjectType, ObjectId: r.ObjectID, ObjectName: r.ObjectName,
			UpBytes: r.UpBytes, DownBytes: r.DownBytes, TotalBytes: r.TotalBytes,
			Connections: r.Connections, LastUpdatedAt: shared.TimeString(r.LastUpdatedAt),
		})
	}
	return &types.TrafficListResp{
		Code: errcode.OK,
		Data: types.TrafficListData{
			Items: items, Total: int(total), Page: req.Page, PageSize: req.PageSize, Unit: "bytes",
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
