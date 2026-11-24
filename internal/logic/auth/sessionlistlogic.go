// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SessionListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 活跃会话
func NewSessionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionListLogic {
	return &SessionListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SessionListLogic) SessionList() (resp *types.SessionListResp, err error) {
	row := auth.SessionFromContext(l.ctx)
	if row == nil {
		return nil, response.Unauthorized()
	}
	store, ok := l.svcCtx.SessionModel.(model.SessionAdminModel)
	if !ok {
		return nil, response.Internal(errors.New("session model does not support listing"))
	}
	sessions, err := store.ListByUserID(l.ctx, row.User.ID)
	if err != nil {
		return nil, response.Internal(err)
	}
	items := make([]types.SessionRow, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, types.SessionRow{
			Id:                s.ID,
			Current:           s.ID == row.Session.ID,
			UserAgent:         s.UserAgent,
			Ip:                s.ClientIP,
			CreatedAt:         shared.TimeString(s.CreatedAt),
			LastActiveAt:      shared.TimeString(s.LastSeenAt),
			IdleExpiresAt:     shared.TimeString(s.ExpiresAt),
			AbsoluteExpiresAt: shared.TimeString(s.AbsoluteExpiresAt),
		})
	}
	return &types.SessionListResp{
		Code:      errcode.OK,
		Data:      types.SessionListData{Items: items},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
