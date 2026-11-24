// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package settings

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type SubscriptionHostDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除订阅域名（幂等）
func NewSubscriptionHostDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubscriptionHostDeleteLogic {
	return &SubscriptionHostDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubscriptionHostDeleteLogic) SubscriptionHostDelete(req *types.GeoFileReq) (resp *types.SubscriptionHostDeleteResp, err error) {
	id, perr := strconv.ParseInt(req.Id, 10, 64)
	if perr == nil && id > 0 {
		if err := l.svcCtx.SubscriptionModel.DeleteHost(l.ctx, id); err != nil {
			return nil, response.Internal(err)
		}
	}
	return &types.SubscriptionHostDeleteResp{
		Code:      errcode.OK,
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
