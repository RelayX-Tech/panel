// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package audit

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

type AuditLogListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 审计列表
func NewAuditLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditLogListLogic {
	return &AuditLogListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuditLogListLogic) AuditLogList(req *types.AuditLogListReq) (resp *types.AuditLogListResp, err error) {
	filter := model.AuditFilter{
		Page: req.Page, PageSize: req.PageSize, Actor: req.Actor, Action: req.Action,
		ObjectType: req.ObjectType, ObjectID: req.ObjectId, RequestID: req.RequestId,
	}
	if req.From != "" {
		at, perr := time.Parse(time.RFC3339, req.From)
		if perr != nil {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "from", "invalid time")
		}
		filter.From = &at
	}
	if req.To != "" {
		at, perr := time.Parse(time.RFC3339, req.To)
		if perr != nil {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "to", "invalid time")
		}
		filter.To = &at
	}
	items, total, err := l.svcCtx.AuditLogModel.List(l.ctx, filter)
	if err != nil {
		return nil, response.Internal(err)
	}
	out := make([]types.AuditLog, 0, len(items))
	for _, a := range items {
		out = append(out, toAuditLog(a))
	}
	return &types.AuditLogListResp{
		Code:      errcode.OK,
		Data:      types.AuditLogListData{Items: out, Total: int(total), Page: req.Page, PageSize: req.PageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}

func toAuditLog(a model.AuditLog) types.AuditLog {
	return types.AuditLog{
		Id: a.ID, Ts: shared.TimeString(a.CreatedAt), Actor: a.Actor, Action: a.Action,
		ObjectType: a.ObjectType, ObjectId: a.ObjectID, Result: a.Result, RequestId: a.RequestID, Ip: a.IP,
	}
}
