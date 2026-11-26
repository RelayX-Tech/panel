// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type NotificationDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除通知渠道
func NewNotificationDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NotificationDeleteLogic {
	return &NotificationDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NotificationDeleteLogic) NotificationDelete(req *types.NotificationTestReq) (resp *types.NotificationDeleteResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_NOTIFICATION, "notification channel not found")
	}
	if err := l.svcCtx.NotificationModel.Delete(l.ctx, id); err != nil {
		return nil, response.Internal(err)
	}
	return &types.NotificationDeleteResp{
		Code:      errcode.OK,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
