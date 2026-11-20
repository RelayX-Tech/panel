// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 注销当前会话（幂等）
func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Logout 吊销 token 对应会话；无会话时同样返回成功（幂等）。
func (l *LogoutLogic) Logout(token string) (resp *types.LogoutResp, err error) {
	if err := l.svcCtx.SessionManager.Revoke(l.ctx, token); err != nil {
		l.Errorw("auth.logout", logx.Field("requestId", requestid.FromContext(l.ctx)),
			logx.Field("result", "failure"))
		return nil, response.Internal(err)
	}
	l.Infow("auth.logout", logx.Field("requestId", requestid.FromContext(l.ctx)),
		logx.Field("result", "success"))
	return &types.LogoutResp{
		Code:      "OK",
		Message:   "",
		Data:      nil,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
