// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return
}
