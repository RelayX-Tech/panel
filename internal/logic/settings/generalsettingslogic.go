// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type GeneralSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 常规设置（只读）
func NewGeneralSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeneralSettingsLogic {
	return &GeneralSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GeneralSettingsLogic) GeneralSettings() (resp *types.GeneralSettingsResp, err error) {
	c := l.svcCtx.Config
	mode := c.Deployment.Mode
	if mode == "" {
		mode = "binary"
	}
	return &types.GeneralSettingsResp{
		Code: errcode.OK,
		Data: types.GeneralSettingsData{
			Listen:                    fmt.Sprintf("%s:%d", c.Host, c.Port),
			PublicUrl:                 "",
			Path:                      "/api/v1",
			SessionIdleTtlSeconds:     int(c.Auth.SessionIdleTTL.Seconds()),
			SessionAbsoluteTtlSeconds: int(c.Auth.SessionAbsoluteTTL.Seconds()),
			DefaultLocale:             "en-US",
			DefaultTheme:              "system",
			PageSize:                  20,
			DeploymentMode:            mode,
			Instances:                 1,
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
