// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package upgrade

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type UpgradeTaskListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 升级历史
func NewUpgradeTaskListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpgradeTaskListLogic {
	return &UpgradeTaskListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpgradeTaskListLogic) UpgradeTaskList(req *types.UpgradeTaskListReq) (resp *types.UpgradeTaskListResp, err error) {
	items, total, err := l.svcCtx.UpgradeModel.ListTasks(l.ctx, model.UpgradeTaskFilter{
		Page: req.Page, PageSize: req.PageSize, Scope: req.Scope, Status: req.Status,
	})
	if err != nil {
		return nil, response.Internal(err)
	}
	out := make([]types.UpgradeTask, 0, len(items))
	for _, t := range items {
		out = append(out, toTask(t))
	}
	return &types.UpgradeTaskListResp{
		Code:      errcode.OK,
		Data:      types.UpgradeTaskListData{Items: out, Total: int(total), Page: req.Page, PageSize: req.PageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
