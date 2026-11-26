// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package alerts

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type AlertListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 告警列表
func NewAlertListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertListLogic {
	return &AlertListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AlertListLogic) AlertList(req *types.AlertListReq) (resp *types.AlertListResp, err error) {
	items, total, err := l.svcCtx.AlertModel.List(l.ctx, model.AlertFilter{
		Page: req.Page, PageSize: req.PageSize, Level: req.Level, Status: req.Status,
		Type: req.AlertType, ObjectType: req.ObjectType, Query: req.Q, Sort: req.Sort,
	})
	if err != nil {
		return nil, response.Internal(err)
	}
	out := make([]types.Alert, 0, len(items))
	for _, a := range items {
		out = append(out, toAlert(a))
	}
	return &types.AlertListResp{
		Code:      errcode.OK,
		Data:      types.AlertListData{Items: out, Total: int(total), Page: req.Page, PageSize: req.PageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
