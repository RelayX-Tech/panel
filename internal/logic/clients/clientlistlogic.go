// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package clients

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type ClientListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 客户端统计
func NewClientListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClientListLogic {
	return &ClientListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClientListLogic) ClientList(req *types.ClientListReq) (resp *types.ClientListResp, err error) {
	row := auth.SessionFromContext(l.ctx)
	if row == nil {
		return nil, response.Unauthorized()
	}
	if roleModel, ok := l.svcCtx.UserModel.(model.RoleModel); ok {
		role, rerr := roleModel.RoleOf(l.ctx, row.User.ID)
		if rerr != nil {
			return nil, response.Internal(rerr)
		}
		if role != "" && role != "admin" {
			return nil, response.NewError(errcode.AUTH_FORBIDDEN, "admin role is required")
		}
	}
	rows, total, err := l.svcCtx.ClientModel.List(l.ctx, model.ClientFilter{
		Page: req.Page, PageSize: req.PageSize, PortGroupID: req.PortGroupId, PortID: req.PortId, Query: req.Q,
	})
	if err != nil {
		return nil, response.Internal(err)
	}
	items := make([]types.ClientRow, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.ClientRow{
			ClientId: r.ClientID, Email: r.Email, PortId: r.PortID, PortGroupId: r.PortGroupID,
			UpBytes: r.UpBytes, DownBytes: r.DownBytes,
			LastOnlineAt: shared.TimeStringOpt(r.LastOnlineAt), LastSubFetchAt: shared.TimeStringOpt(r.LastSubFetchAt),
		})
	}
	return &types.ClientListResp{
		Code:      errcode.OK,
		Data:      types.ClientListData{Items: items, Total: int(total), Page: req.Page, PageSize: req.PageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
