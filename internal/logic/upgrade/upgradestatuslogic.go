// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package upgrade

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type UpgradeStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 升级状态与锁
func NewUpgradeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradeStatusLogic {
	return &UpgradeStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradeStatusLogic) UpgradeStatus() (resp *types.UpgradeStatusResp, err error) {
	if l.svcCtx.Upgrade == nil {
		return nil, nil
	}
	view, err := l.svcCtx.Upgrade.Status(l.ctx)
	if err != nil {
		return nil, mapError(err)
	}
	data := types.UpgradeStatusData{
		Panel: types.UpgradePanelInfo{
			CurrentVersion: view.Current, LatestVersion: view.Latest, UpdateAvailable: view.Latest != "" && view.Latest != view.Current,
			Channel: view.Channel, ComposeMode: view.Compose,
		},
		Compatibility: compatibilityInfo(),
	}
	if view.Lock != nil {
		data.Lock = &types.UpgradeLockInfo{
			Holder: view.Lock.Holder, Scope: view.Lock.Scope,
			ExpiresAt: view.Lock.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		}
	}
	if view.Active != nil {
		task := toTask(*view.Active)
		data.ActiveTask = &task
	}
	return &types.UpgradeStatusResp{
		Code:      errcode.OK,
		Data:      data,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
