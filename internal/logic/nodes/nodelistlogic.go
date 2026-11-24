// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package nodes

import (
	"context"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type NodeListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 节点列表
func NewNodeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeListLogic {
	return &NodeListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeListLogic) NodeList(req *types.NodeListReq) (resp *types.NodeListResp, err error) {
	page, pageSize := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	switch req.Sort {
	case "", "status", "cpu", "mem", "lastSeen":
	default:
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_SORT, "sort", "unsupported sort field")
	}

	filter := model.NodeFilter{
		Page: page, PageSize: pageSize, Type: req.NodeType, Status: req.Status,
		GroupID: req.GroupId, Region: req.Region, Query: req.Q, Sort: req.Sort,
	}
	displayFilter := ""
	switch req.Status {
	case "upgrading", "pending_upgrade":
		displayFilter = req.Status
		filter.Status = ""
	}

	states := l.displayStates(l.ctx)

	var items []model.NodeSummary
	var total int64
	if displayFilter == "" {
		items, total, err = l.svcCtx.NodeModel.List(l.ctx, filter)
		if err != nil {
			return nil, response.Internal(err)
		}
	} else {
		// 派生态不是数据库列：先取候选集，再按展示态过滤与分页（上限 1000）。
		all := make([]model.NodeSummary, 0)
		for p := 1; p <= 10; p++ {
			pageItems, _, lerr := l.svcCtx.NodeModel.List(l.ctx, model.NodeFilter{
				Page: p, PageSize: 100, Type: req.NodeType, GroupID: req.GroupId,
				Region: req.Region, Query: req.Q, Sort: req.Sort,
			})
			if lerr != nil {
				return nil, response.Internal(lerr)
			}
			all = append(all, pageItems...)
			if len(pageItems) < 100 {
				break
			}
		}
		filtered := make([]model.NodeSummary, 0)
		for _, n := range all {
			if states[n.ID] == displayFilter {
				filtered = append(filtered, n)
			}
		}
		total = int64(len(filtered))
		start := (page - 1) * pageSize
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + pageSize
		if end > len(filtered) {
			end = len(filtered)
		}
		items = filtered[start:end]
	}

	out := make([]types.NodeSummary, 0, len(items))
	for _, n := range items {
		out = append(out, toNodeSummary(n, states[n.ID]))
	}
	return &types.NodeListResp{
		Code:      errcode.OK,
		Data:      types.NodeListData{Items: out, Total: int(total), Page: page, PageSize: pageSize},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}

// normalizeIDList 解析逗号分隔的 ID 列表。
func normalizeIDList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
