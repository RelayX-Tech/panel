// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/notify"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

var errNoMasterKey = errors.New("RELAYX_MASTER_KEY is not configured")

type NotificationTestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 发送测试通知
func NewNotificationTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NotificationTestLogic {
	return &NotificationTestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NotificationTestLogic) NotificationTest(req *types.NotificationTestReq) (resp *types.NotificationTestResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_NOTIFICATION, "notification channel not found")
	}
	channel, err := l.svcCtx.NotificationModel.Get(l.ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_NOTIFICATION, "notification channel not found")
		}
		return nil, response.Internal(err)
	}
	if l.svcCtx.Box == nil {
		return nil, response.Internal(errNoMasterKey)
	}
	plaintext, err := l.svcCtx.Box.Open(channel.TargetEncrypted)
	if err != nil {
		return nil, response.Internal(err)
	}
	sender := notify.NewClient()
	if err := sender.Send(l.ctx, channel.Type, string(plaintext), "relayx panel test notification"); err != nil {
		l.Errorw("settings.notification.test", logx.Field("channelId", id), logx.Field("error", err.Error()))
		return nil, response.NewError(errcode.UPSTREAM_NOTIFICATION_FAILED, "notification delivery failed")
	}
	_ = l.svcCtx.NotificationModel.MarkSent(l.ctx, id, time.Now().UTC())
	return &types.NotificationTestResp{
		Code:      errcode.OK,
		Data:      types.NotificationTestData{Success: true},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
