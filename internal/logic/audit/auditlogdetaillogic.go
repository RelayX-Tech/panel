// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package audit

import (
	"context"
	"errors"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type AuditLogDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 审计详情
func NewAuditLogDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditLogDetailLogic {
	return &AuditLogDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuditLogDetailLogic) AuditLogDetail(req *types.AuditLogDetailReq) (resp *types.AuditLogDetailResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_AUDIT_LOG, "audit log not found")
	}
	item, err := l.svcCtx.AuditLogModel.Get(l.ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_AUDIT_LOG, "audit log not found")
		}
		return nil, response.Internal(err)
	}
	return &types.AuditLogDetailResp{
		Code:      errcode.OK,
		Data:      types.AuditLogDetailData{Item: toAuditLog(*item)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
