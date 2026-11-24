// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SubscriptionHostListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 订阅域名映射列表
func NewSubscriptionHostListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscriptionHostListLogic {
	return &SubscriptionHostListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscriptionHostListLogic) SubscriptionHostList(req *types.SubscriptionHostListReq) (resp *types.SubscriptionHostListResp, err error) {
	items, total, err := l.svcCtx.SubscriptionModel.ListHosts(l.ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, response.Internal(err)
	}
	out := make([]types.SubscriptionHost, 0, len(items))
	for _, h := range items {
		out = append(out, toSubscriptionHost(h))
	}
	return &types.SubscriptionHostListResp{
		Code: errcode.OK,
		Data: types.SubscriptionHostListData{
			Items: out, Total: int(total), Page: req.Page, PageSize: req.PageSize,
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
