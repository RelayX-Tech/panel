// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type ProfileUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新显示名
func NewProfileUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileUpdateLogic {
	return &ProfileUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfileUpdateLogic) ProfileUpdate(req *types.ProfileUpdateReq) (resp *types.ProfileResp, err error) {
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "displayName", "displayName is required")
	}
	if utf8.RuneCountInString(name) > 64 {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "displayName", "displayName must be at most 64 characters")
	}
	row := auth.SessionFromContext(l.ctx)
	if row == nil {
		return nil, response.Unauthorized()
	}
	updater, ok := l.svcCtx.UserModel.(model.DisplayNameUpdater)
	if !ok {
		return nil, response.Internal(errors.New("user model does not support display name updates"))
	}
	if err := updater.UpdateDisplayName(l.ctx, nil, row.User.ID, name); err != nil {
		return nil, response.Internal(err)
	}
	if l.svcCtx.AuditLogModel != nil {
		_ = l.svcCtx.AuditLogModel.Insert(l.ctx, model.AuditLog{
			Actor: row.User.Account, Action: "profile.update", ObjectType: "user", ObjectID: row.User.ID,
			Result: "success", RequestID: requestid.FromContext(l.ctx), CreatedAt: time.Now().UTC(),
		})
	}
	return &types.ProfileResp{
		Code:      errcode.OK,
		Data:      types.ProfileData{Id: row.User.ID, Account: row.User.Account, DisplayName: name},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
