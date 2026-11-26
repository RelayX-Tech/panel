// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SecuritySettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 安全策略（只读）
func NewSecuritySettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecuritySettingsLogic {
	return &SecuritySettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SecuritySettingsLogic) SecuritySettings() (resp *types.SecuritySettingsResp, err error) {
	c := l.svcCtx.Config
	return &types.SecuritySettingsResp{
		Code: errcode.OK,
		Data: types.SecuritySettingsData{
			SessionIdleTtlSeconds:     int(c.Auth.SessionIdleTTL.Seconds()),
			SessionAbsoluteTtlSeconds: int(c.Auth.SessionAbsoluteTTL.Seconds()),
			LoginRateLimit: types.RateLimitSpec{
				WindowSeconds: int(c.Auth.LoginRateLimit.Window.Seconds()),
				MaxAttempts:   c.Auth.LoginRateLimit.MaxAttempts,
			},
			PasswordRateLimit: types.RateLimitSpec{
				WindowSeconds: int(c.Auth.PasswordChangeRateLimit.Window.Seconds()),
				MaxAttempts:   c.Auth.PasswordChangeRateLimit.MaxAttempts,
			},
			AllowedOrigins:         append([]string{}, c.Auth.AllowedOrigins...),
			CookieSecure:           c.Auth.CookieSecure,
			TrustedProxyCidrs:      append([]string{}, c.Auth.TrustedProxyCIDRs...),
			CertFingerprintPinning: false,
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
