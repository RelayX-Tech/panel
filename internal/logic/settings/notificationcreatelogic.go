// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/notify"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type NotificationCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新增通知渠道
func NewNotificationCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NotificationCreateLogic {
	return &NotificationCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NotificationCreateLogic) NotificationCreate(req *types.NotificationCreateReq) (resp *types.NotificationItemResp, err error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, response.MissingField("name")
	}
	if !validChannelType(req.ChannelType) {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "type", "type must be telegram or webhook")
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, response.MissingField("target")
	}
	level := strings.ToLower(strings.TrimSpace(req.MinLevel))
	if !validLevel(level) {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "minLevel", "minLevel must be info, warning or critical")
	}
	if l.svcCtx.Box == nil {
		return nil, response.Internal(errNoMasterKey)
	}
	sealed, err := l.svcCtx.Box.Seal([]byte(target))
	if err != nil {
		return nil, response.Internal(err)
	}
	created, err := l.svcCtx.NotificationModel.Create(l.ctx, model.NotificationChannel{
		Name: name, Type: strings.ToLower(req.ChannelType), TargetEncrypted: sealed,
		TargetMasked: notify.Mask(target), MinLevel: level, Enabled: req.Enabled,
	})
	if err != nil {
		return nil, response.Internal(err)
	}
	return &types.NotificationItemResp{
		Code:      errcode.OK,
		Data:      types.NotificationItemData{Item: toNotificationChannel(*created)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
