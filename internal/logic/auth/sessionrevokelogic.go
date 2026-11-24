// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/middleware"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SessionRevokeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 吊销其他会话
func NewSessionRevokeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionRevokeLogic {
	return &SessionRevokeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SessionRevokeLogic) SessionRevoke(req *types.SessionRevokeReq) (resp *types.SessionRevokeResp, err error) {
	row := auth.SessionFromContext(l.ctx)
	if row == nil {
		return nil, response.Unauthorized()
	}
	if req.Id == row.Session.ID {
		return nil, response.NewError(errcode.FORBIDDEN_SELF_SESSION_REVOKE, "cannot revoke the current session")
	}
	store, ok := l.svcCtx.SessionModel.(model.SessionAdminModel)
	if !ok {
		return nil, response.Internal(errors.New("session model does not support revocation"))
	}
	target, err := store.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NOT_FOUND_SESSION, "session not found")
		}
		return nil, response.Internal(err)
	}
	if target.UserID != row.User.ID {
		return nil, response.NewError(errcode.NOT_FOUND_SESSION, "session not found")
	}
	if err := store.RevokeByID(l.ctx, nil, target.ID); err != nil {
		return nil, response.Internal(err)
	}
	if l.svcCtx.AuditLogModel != nil {
		_ = l.svcCtx.AuditLogModel.Insert(l.ctx, model.AuditLog{
			Actor: row.User.Account, Action: "session.revoke", ObjectType: "session", ObjectID: target.ID,
			Result: "success", RequestID: requestid.FromContext(l.ctx),
			IP: middleware.ClientIPFromContext(l.ctx), CreatedAt: time.Now().UTC(),
		})
	}
	return &types.SessionRevokeResp{
		Code:      errcode.OK,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
