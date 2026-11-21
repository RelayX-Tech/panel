// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 读取当前会话
func NewSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionLogic {
	return &SessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Session 返回 Auth 中间件注入的当前会话与用户。
func (l *SessionLogic) Session() (resp *types.SessionResp, err error) {
	row := auth.SessionFromContext(l.ctx)
	if row == nil {
		return nil, response.Unauthorized()
	}
	return &types.SessionResp{
		Code:    "OK",
		Message: "",
		Data: types.SessionData{
			User: types.UserInfo{
				Id:          row.User.ID,
				Account:     row.User.Account,
				DisplayName: row.User.DisplayName,
			},
			Session: types.SessionInfo{
				ExpiresAt:         formatTime(row.Session.ExpiresAt),
				AbsoluteExpiresAt: formatTime(row.Session.AbsoluteExpiresAt),
			},
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
