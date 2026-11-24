// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package ws

import (
	"context"

	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// WebSocket 实时通道
func NewWsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WsLogic {
	return &WsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WsLogic) Ws(req *types.WsReq) error {
	// todo: add your logic here and delete this line

	return nil
}
