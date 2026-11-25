// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package upgrade

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/relayx-tech/panel/internal/upgrade"
)

type UpgradeCheckLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 手动检查更新
func NewUpgradeCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradeCheckLogic {
	return &UpgradeCheckLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradeCheckLogic) UpgradeCheck() (resp *types.UpgradeCheckResp, err error) {
	data := types.UpgradeCheckData{CheckedAt: shared.TimeString(nowUTC())}
	if l.svcCtx.Upgrade == nil {
		data.ErrorCode = errcode.UPSTREAM_UPDATE_SOURCE_FAILED
		return &types.UpgradeCheckResp{Code: errcode.OK, Data: data, RequestId: requestid.FromContext(l.ctx)}, nil
	}
	result, checkErr := l.svcCtx.Upgrade.Check(l.ctx)
	if result != nil {
		data.CheckedAt = shared.TimeString(result.CheckedAt)
		data.LatestVersion = result.LatestVersion
		data.UpdateAvailable = result.UpdateAvailable
	}
	if checkErr != nil {
		if !errors.Is(checkErr, upgrade.ErrUpdateSourceNotConfigured) {
			// 已配置但检查失败：保留上次结果并给出错误码；未配置不视为故障。
			data.ErrorCode = errcode.UPSTREAM_UPDATE_SOURCE_FAILED
			l.Errorw("upgrade.check.failure", logx.Field("error", checkErr.Error()))
		}
	}
	return &types.UpgradeCheckResp{
		Code:      errcode.OK,
		Data:      data,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
