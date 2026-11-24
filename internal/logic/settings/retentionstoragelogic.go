// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type RetentionStorageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 存储用量
func NewRetentionStorageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetentionStorageLogic {
	return &RetentionStorageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetentionStorageLogic) RetentionStorage() (resp *types.RetentionStorageResp, err error) {
	stats, err := l.svcCtx.TrafficModel.StorageStats(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	data := types.RetentionStorageData{
		DatabaseSizeBytes:         stats.DatabaseSizeBytes,
		TrafficPartitionSizeBytes: stats.TrafficPartitionSizeBytes,
		CollectedAt:               shared.TimeString(time.Now().UTC()),
	}
	if stats.EarliestRecordAt != nil {
		v := shared.TimeString(*stats.EarliestRecordAt)
		data.EarliestRecordAt = &v
	}
	if stats.LatestRecordAt != nil {
		v := shared.TimeString(*stats.LatestRecordAt)
		data.LatestRecordAt = &v
	}
	return &types.RetentionStorageResp{
		Code:      errcode.OK,
		Data:      data,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
