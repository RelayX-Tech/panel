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

type UpgradeAgentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 发起 Agent 单节点或批次升级
func NewUpgradeAgentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradeAgentsLogic {
	return &UpgradeAgentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradeAgentsLogic) UpgradeAgents(req *types.UpgradeAgentReq) (resp *types.UpgradeTaskResp, err error) {
	if req.TargetVersion == "" {
		return nil, response.MissingField("targetVersion")
	}
	hasNodes := len(req.NodeIds) > 0
	hasGroup := req.GroupId != ""
	if hasNodes == hasGroup {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "nodeIds", "nodeIds and groupId are mutually exclusive and one is required")
	}
	if hasGroup && req.GroupType != "entry" && req.GroupType != "tunnel" {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "groupType", "groupType must be entry or tunnel")
	}
	if req.OperationId == "" {
		req.OperationId = requestid.FromContext(l.ctx)
	}
	if l.svcCtx.Upgrade == nil {
		return nil, response.NewError(errcode.UPSTREAM_AGENT_UNAVAILABLE, "agent upgrade is not configured")
	}
	task, err := l.svcCtx.Upgrade.StartAgents(l.ctx, actorFrom(l.ctx), requestid.FromContext(l.ctx),
		req.OperationId, req.TargetVersion, req.NodeIds, req.GroupId, req.GroupType)
	if err != nil {
		return nil, mapError(err)
	}
	return &types.UpgradeTaskResp{
		Code:      errcode.OK,
		Data:      types.UpgradeTaskData{Item: toTask(*task)},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
