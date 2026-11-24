// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type ProbeApplyStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 探测规格下发进度
func NewProbeApplyStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProbeApplyStatusLogic {
	return &ProbeApplyStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProbeApplyStatusLogic) ProbeApplyStatus() (resp *types.ProbeApplyStatusResp, err error) {
	data := types.ProbeApplyStatusData{State: "idle", Nodes: make([]types.ProbeApplyNode, 0)}
	if l.svcCtx.SettingsApplyModel != nil {
		task, err := l.svcCtx.SettingsApplyModel.LatestTask(l.ctx, "probe")
		if err != nil {
			return nil, response.Internal(err)
		}
		if task != nil {
			data.TaskId = task.ID
			data.State = task.State
			data.UpdatedAt = shared.TimeString(task.UpdatedAt)
			for _, n := range task.Nodes {
				data.Nodes = append(data.Nodes, types.ProbeApplyNode{
					NodeId: n.NodeID, NodeName: n.NodeName, State: n.State,
					Version: n.Version, ElapsedMs: n.ElapsedMs, ErrorCode: n.ErrorCode,
				})
			}
		}
	}
	return &types.ProbeApplyStatusResp{
		Code:      errcode.OK,
		Data:      data,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
