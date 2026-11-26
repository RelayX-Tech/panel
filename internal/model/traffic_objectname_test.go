package model_test

import (
	"context"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/testsupport"
)

// TestTrafficRowsUseDisplayName 覆盖 C19：/traffic 的 TrafficRow.objectName
// 应回填实体显示名（端口组 / 节点），无法解析时回退 entity_id。
func TestTrafficRowsUseDisplayName(t *testing.T) {
	db := newDB(t)
	m := model.NewTrafficModel(testsupport.Conn(db))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute)
	from, to := now.Add(-time.Hour), now.Add(time.Minute)

	if _, err := db.Exec(`INSERT INTO port_groups (id, name) VALUES ('pg-hk-jp', 'HK-JP 专线')`); err != nil {
		t.Fatalf("seed port group: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO nodes (id, name) VALUES ('n1', 'node-1')`); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	for _, s := range []model.TrafficSample{
		{EntityType: "port_group", EntityID: "pg-hk-jp", TS: now, UpBytes: 10, DownBytes: 20, Connections: 1},
		{EntityType: "port_group", EntityID: "pg-unknown", TS: now, UpBytes: 5, DownBytes: 5, Connections: 1},
		{EntityType: "node", EntityID: "n1", TS: now, UpBytes: 7, DownBytes: 7, Connections: 1},
	} {
		if err := m.UpsertSample(ctx, s); err != nil {
			t.Fatalf("upsert sample %s/%s: %v", s.EntityType, s.EntityID, err)
		}
	}

	groups, _, err := m.Rows(ctx, "port_group", from, to, 1, 20, "")
	if err != nil {
		t.Fatalf("rows port_group: %v", err)
	}
	byID := make(map[string]model.TrafficRow, len(groups))
	for _, r := range groups {
		byID[r.ObjectID] = r
	}
	if got := byID["pg-hk-jp"].ObjectName; got != "HK-JP 专线" {
		t.Errorf("port_group objectName = %q, want display name %q", got, "HK-JP 专线")
	}
	if got := byID["pg-unknown"].ObjectName; got != "pg-unknown" {
		t.Errorf("unknown port_group objectName = %q, want fallback to entity_id", got)
	}

	nodes, _, err := m.Rows(ctx, "node", from, to, 1, 20, "")
	if err != nil {
		t.Fatalf("rows node: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ObjectName != "node-1" {
		t.Errorf("node rows = %+v, want single row with objectName node-1", nodes)
	}
}
