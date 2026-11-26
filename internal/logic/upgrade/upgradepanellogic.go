// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package upgrade

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type UpgradePanelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 发起 Panel 升级
func NewUpgradePanelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradePanelLogic {
	return &UpgradePanelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradePanelLogic) UpgradePanel(req *types.UpgradePanelReq) (resp *types.UpgradeTaskResp, err error) {
	if req.TargetVersion == "" {
		return nil, response.MissingField("targetVersion")
	}
	if req.OperationId == "" {
		req.OperationId = requestid.FromContext(l.ctx)
	}
	if l.svcCtx.Upgrade == nil {
		return nil, response.NewError(errcode.UPSTREAM_UPGRADER_UNAVAILABLE, "upgrader is not configured")
	}
	task, err := l.svcCtx.Upgrade.StartPanel(l.ctx, actorFrom(l.ctx), requestid.FromContext(l.ctx), req.OperationId, req.TargetVersion)
	if err != nil {
		return nil, mapError(err)
	}
	return &types.UpgradeTaskResp{
		Code:      errcode.OK,
		Data:      types.UpgradeTaskData{Item: toTask(*task)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
