// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package nodes

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type NodeDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 节点详情头部
func NewNodeDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeDetailLogic {
	return &NodeDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeDetailLogic) NodeDetail(req *types.NodeDetailReq) (resp *types.NodeDetailResp, err error) {
	detail, err := l.svcCtx.NodeModel.Get(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NODE_NOT_FOUND, "node not found")
		}
		return nil, response.Internal(err)
	}
	list := &NodeListLogic{Logger: l.Logger, ctx: l.ctx, svcCtx: l.svcCtx}
	states := list.displayStates(l.ctx)
	return &types.NodeDetailResp{
		Code:      errcode.OK,
		Data:      toNodeDetail(detail, states[detail.ID]),
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
