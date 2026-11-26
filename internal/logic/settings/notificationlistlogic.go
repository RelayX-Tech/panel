// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type NotificationListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通知渠道列表
func NewNotificationListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NotificationListLogic {
	return &NotificationListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NotificationListLogic) NotificationList() (resp *types.NotificationListResp, err error) {
	channels, err := l.svcCtx.NotificationModel.List(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	items := make([]types.NotificationChannel, 0, len(channels))
	for _, c := range channels {
		items = append(items, toNotificationChannel(c))
	}
	return &types.NotificationListResp{
		Code:      errcode.OK,
		Data:      types.NotificationListData{Items: items},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
