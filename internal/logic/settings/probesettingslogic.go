// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type ProbeSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 探测规格
func NewProbeSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProbeSettingsLogic {
	return &ProbeSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProbeSettingsLogic) ProbeSettings() (resp *types.ProbeSettingsResp, err error) {
	settings, err := loadProbeSettings(l.ctx, l.svcCtx.SettingModel)
	if err != nil {
		return nil, response.Internal(err)
	}
	return &types.ProbeSettingsResp{
		Code: errcode.OK,
		Data: types.ProbeSettingsData{
			FrequencySeconds:  settings.FrequencySeconds,
			TimeoutMs:         settings.TimeoutMs,
			FailureThreshold:  settings.FailureThreshold,
			RecoveryThreshold: settings.RecoveryThreshold,
			UpdatedAt:         settings.UpdatedAt,
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
