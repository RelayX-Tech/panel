// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"errors"
	"strconv"
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

type NotificationUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 编辑通知渠道
func NewNotificationUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NotificationUpdateLogic {
	return &NotificationUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NotificationUpdateLogic) NotificationUpdate(req *types.NotificationUpdateReq) (resp *types.NotificationItemResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_NOTIFICATION, "notification channel not found")
	}
	patch := model.NotificationPatch{Name: req.Name, MinLevel: req.MinLevel, Enabled: req.Enabled}
	if req.MinLevel != nil {
		level := strings.ToLower(strings.TrimSpace(*req.MinLevel))
		if !validLevel(level) {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "minLevel", "minLevel must be info, warning or critical")
		}
		patch.MinLevel = &level
	}
	if req.Target != nil {
		target := strings.TrimSpace(*req.Target)
		if target == "" {
			return nil, response.MissingField("target")
		}
		if l.svcCtx.Box == nil {
			return nil, response.Internal(errNoMasterKey)
		}
		sealed, err := l.svcCtx.Box.Seal([]byte(target))
		if err != nil {
			return nil, response.Internal(err)
		}
		masked := notify.Mask(target)
		patch.TargetEncrypted = sealed
		patch.TargetMasked = &masked
	}
	updated, err := l.svcCtx.NotificationModel.Update(l.ctx, id, patch)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_NOTIFICATION, "notification channel not found")
		}
		return nil, response.Internal(err)
	}
	return &types.NotificationItemResp{
		Code:      errcode.OK,
		Data:      types.NotificationItemData{Item: toNotificationChannel(*updated)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
