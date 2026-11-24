// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package nodes

import (
	"context"
	"encoding/json"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type NodeEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 节点事件（30 天）
func NewNodeEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeEventsLogic {
	return &NodeEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeEventsLogic) NodeEvents(req *types.NodeEventsReq) (resp *types.NodeEventsResp, err error) {
	filter := model.EventFilter{Page: req.Page, PageSize: req.PageSize, Type: req.EventType}
	if req.From != "" {
		at, perr := time.Parse(time.RFC3339, req.From)
		if perr != nil {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "from", "invalid time")
		}
		filter.From = &at
	}
	if req.To != "" {
		at, perr := time.Parse(time.RFC3339, req.To)
		if perr != nil {
			return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "to", "invalid time")
		}
		filter.To = &at
	}
	items, total, err := l.svcCtx.NodeModel.Events(l.ctx, req.Id, filter)
	if err != nil {
		return nil, response.Internal(err)
	}
	out := make([]types.NodeEvent, 0, len(items))
	for _, e := range items {
		var metadata interface{}
		if len(e.Metadata) > 0 {
			var v map[string]any
			if json.Unmarshal(e.Metadata, &v) == nil {
				metadata = v
			}
		}
		out = append(out, types.NodeEvent{
			Id: e.ID, NodeId: e.NodeID, EventType: e.Type, Level: e.Level,
			Message: e.Message, Metadata: metadata, OccurredAt: shared.TimeString(e.OccurredAt),
		})
	}
	return &types.NodeEventsResp{
		Code: errcode.OK,
		Data: types.NodeEventsData{
			Items: out, Total: int(total),
			Page: req.Page, PageSize: req.PageSize,
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
