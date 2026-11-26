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

type UpgradeTaskDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 任务详情
func NewUpgradeTaskDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradeTaskDetailLogic {
	return &UpgradeTaskDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradeTaskDetailLogic) UpgradeTaskDetail(req *types.UpgradeTaskDetailReq) (resp *types.UpgradeTaskDetailResp, err error) {
	task, err := l.svcCtx.UpgradeModel.GetTask(l.ctx, req.Id)
	if err != nil {
		return nil, mapError(err)
	}
	return &types.UpgradeTaskDetailResp{
		Code:      errcode.OK,
		Data:      types.UpgradeTaskDetailData{Item: toTask(*task)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
