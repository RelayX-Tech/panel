// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package alerts

import (
	"context"
	"errors"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type AlertDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 告警详情
func NewAlertDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertDetailLogic {
	return &AlertDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AlertDetailLogic) AlertDetail(req *types.AlertDetailReq) (resp *types.AlertDetailResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr != nil || id <= 0 {
		return nil, response.NewError(errcode.NOT_FOUND_ALERT, "alert not found")
	}
	alert, err := l.svcCtx.AlertModel.Get(l.ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_ALERT, "alert not found")
		}
		return nil, response.Internal(err)
	}
	notifications, err := l.svcCtx.AlertModel.Notifications(l.ctx, id)
	if err != nil {
		return nil, response.Internal(err)
	}
	history := make([]types.AlertNotificationInfo, 0, len(notifications))
	for _, n := range notifications {
		history = append(history, types.AlertNotificationInfo{
			Status: n.Status, Channel: strconv.FormatInt(n.ChannelID, 10),
			ErrorCode: n.ErrorCode, SentAt: shared.TimeString(n.SentAt),
		})
	}
	return &types.AlertDetailResp{
		Code:      errcode.OK,
		Data:      types.AlertDetailData{Alert: toAlert(*alert), NotificationHistory: history},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
