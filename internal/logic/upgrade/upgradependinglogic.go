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

type UpgradePendingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 待升级清单
func NewUpgradePendingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradePendingLogic {
	return &UpgradePendingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradePendingLogic) UpgradePending() (resp *types.UpgradePendingResp, err error) {
	targets, err := l.svcCtx.UpgradeModel.ListTargets(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	items := make([]types.UpgradePendingItem, 0, len(targets))
	for _, t := range targets {
		items = append(items, types.UpgradePendingItem{
			NodeId: t.NodeID, NodeName: t.NodeName, TargetVersion: t.TargetVersion, BatchId: t.BatchID,
			CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
			ExpiresAt: t.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		})
	}
	return &types.UpgradePendingResp{
		Code:      errcode.OK,
		Data:      types.UpgradePendingData{Items: items},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
