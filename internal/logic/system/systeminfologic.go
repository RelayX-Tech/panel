// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package system

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/system"
	"github.com/relayx-tech/panel/internal/types"
)

type SystemInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 版本与兼容信息
func NewSystemInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemInfoLogic {
	return &SystemInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SystemInfoLogic) SystemInfo() (resp *types.SystemInfoResp, err error) {
	versions, err := l.svcCtx.NodeModel.ComponentVersions(l.ctx)
	if err != nil {
		return nil, response.Internal(err)
	}
	return &types.SystemInfoResp{
		Code: errcode.OK,
		Data: types.SystemInfoData{
			Panel: types.SystemPanel{
				Version: system.Version, Commit: system.Commit, BuildTime: system.BuildTime,
				UptimeSeconds: int64(timeSinceStart().Seconds()),
			},
			Components: types.SystemComponents{
				CoreControl: dashIfEmpty(system.CoreControlVersion),
				CoreNodeMin: dashIfEmpty(system.CoreNodeMin),
				CoreNodeMax: dashIfEmpty(system.CoreNodeMax),
				AgentMin:    dashIfEmpty(versions.AgentMin),
				AgentMax:    dashIfEmpty(versions.AgentMax),
				Contract:    system.ContractVersion,
				Schema:      system.SchemaVersion,
				Xray:        dashIfEmpty(firstNonEmpty(versions.Xray, system.XrayBaseline)),
			},
			Compatibility: types.CompatibilityInfo{
				ContractMajor:   system.ContractMajor,
				SchemaVersion:   system.SchemaVersion,
				AgentMinVersion: versions.AgentMin,
				AgentMaxVersion: versions.AgentMax,
				Compatible:      true,
			},
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
