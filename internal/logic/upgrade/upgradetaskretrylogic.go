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

type UpgradeTaskRetryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 手动重试失败任务
func NewUpgradeTaskRetryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradeTaskRetryLogic {
	return &UpgradeTaskRetryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradeTaskRetryLogic) UpgradeTaskRetry(req *types.UpgradeTaskRetryReq) (resp *types.UpgradeTaskResp, err error) {
	if l.svcCtx.Upgrade == nil {
		return nil, response.NewError(errcode.UPSTREAM_UPGRADER_UNAVAILABLE, "upgrade is not configured")
	}
	task, err := l.svcCtx.Upgrade.Retry(l.ctx, actorFrom(l.ctx), requestid.FromContext(l.ctx), req.Id)
	if err != nil {
		return nil, mapError(err)
	}
	return &types.UpgradeTaskResp{
		Code:      errcode.OK,
		Data:      types.UpgradeTaskData{Item: toTask(*task)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
