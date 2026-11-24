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

type RetentionSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保留策略（只读）
func NewRetentionSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetentionSettingsLogic {
	return &RetentionSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetentionSettingsLogic) RetentionSettings() (resp *types.RetentionSettingsResp, err error) {
	return &types.RetentionSettingsResp{
		Code: errcode.OK,
		Data: types.RetentionSettingsData{
			RawDays: 7, MinuteDays: 30, HourDays: 365,
			SnapshotPerNode: 20, SnapshotPerPortGroup: 50, SnapshotDays: 30,
			CleanupAtUtc: "03:00", AuditDays: 180,
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
