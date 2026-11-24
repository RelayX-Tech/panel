package nodes_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/nodes"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

type fakeNodeModel struct {
	model.NodeModel
	items []model.NodeSummary
	total int64
}

func (f fakeNodeModel) List(context.Context, model.NodeFilter) ([]model.NodeSummary, int64, error) {
	return f.items, f.total, nil
}
func (f fakeNodeModel) Get(context.Context, string) (*model.NodeDetail, error) {
	return nil, model.ErrNotFound
}
func (f fakeNodeModel) Events(context.Context, string, model.EventFilter) ([]model.NodeEvent, int64, error) {
	return nil, 0, nil
}

func errCode(err error) string {
	var appErr *response.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func TestNodeList(t *testing.T) {
	now := time.Now().UTC()
	svcCtx := &svc.ServiceContext{NodeModel: fakeNodeModel{
		items: []model.NodeSummary{{ID: "n1", Name: "node-1", Status: "online", LastSeenAt: &now}},
		total: 1,
	}}
	l := nodes.NewNodeListLogic(context.Background(), svcCtx)
	resp, err := l.NodeList(&types.NodeListReq{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Data.Total != 1 || resp.Data.Items[0].Name != "node-1" || resp.Data.Items[0].LastSeenAt == "" {
		t.Fatalf("unexpected list: %+v", resp.Data)
	}

	if _, err := l.NodeList(&types.NodeListReq{Page: 1, PageSize: 20, Sort: "password"}); errCode(err) != "VALIDATION_INVALID_SORT" {
		t.Fatalf("invalid sort code = %q, want VALIDATION_INVALID_SORT", errCode(err))
	}
}

func TestNodeDetailNotFound(t *testing.T) {
	l := nodes.NewNodeDetailLogic(context.Background(), &svc.ServiceContext{NodeModel: fakeNodeModel{}})
	_, err := l.NodeDetail(&types.NodeDetailReq{Id: "missing"})
	if errCode(err) != "NODE_NOT_FOUND" {
		t.Fatalf("code = %q, want NODE_NOT_FOUND", errCode(err))
	}
}
