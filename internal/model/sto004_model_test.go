package model_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testsupport.NewPostgres(t)
	if err := migrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestNodeModel(t *testing.T) {
	db := newDB(t)
	m := model.NewNodeModel(testsupport.Conn(db))
	ctx := context.Background()

	if _, err := db.Exec(`INSERT INTO nodes (id, name, type, group_id, region, status, ip, agent_version, last_seen_at, cpu_percent, mem_percent)
VALUES ('n1', 'node-1', 'entry', 'g1', 'cn-east', 'online', '10.0.0.1', '1.2.0', now(), 12.5, 33.0)`); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	items, total, err := m.List(ctx, model.NodeFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Name != "node-1" || items[0].AgentVersion != "1.2.0" {
		t.Fatalf("unexpected list result: total=%d items=%+v", total, items)
	}

	filtered, total, err := m.List(ctx, model.NodeFilter{Page: 1, PageSize: 20, Status: "offline"})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if total != 0 || len(filtered) != 0 {
		t.Fatalf("status filter should return none, got %d", total)
	}

	detail, err := m.Get(ctx, "n1")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if detail.CPUPercent != 12.5 || detail.GroupID != "g1" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if _, err := m.Get(ctx, "missing"); err != model.ErrNotFound {
		t.Fatalf("get missing = %v, want ErrNotFound", err)
	}

	if err := m.InsertEvent(ctx, &model.NodeEvent{NodeID: "n1", Type: "online", Level: "info", Message: "back online", OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	events, eventTotal, err := m.Events(ctx, "n1", model.EventFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if eventTotal != 1 || events[0].Type != "online" {
		t.Fatalf("unexpected events: %d %+v", eventTotal, events)
	}
}

func TestAlertModel(t *testing.T) {
	db := newDB(t)
	m := model.NewAlertModel(testsupport.Conn(db))
	ctx := context.Background()
	now := time.Now().UTC()

	created, err := m.Upsert(ctx, model.Alert{
		Type: "node_offline", Level: "critical", ObjectType: "node", ObjectID: "n1",
		Message: "node offline", Status: model.AlertStatusActive, FirstSeenAt: now, LastSeenAt: now,
	})
	if err != nil {
		t.Fatalf("upsert alert: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created alert has zero id")
	}

	again, err := m.Upsert(ctx, model.Alert{
		Type: "node_offline", Level: "critical", ObjectType: "node", ObjectID: "n1",
		Message: "node offline again", Status: model.AlertStatusActive, FirstSeenAt: now, LastSeenAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("upsert existing alert: %v", err)
	}
	if again.ID != created.ID {
		t.Fatalf("upsert should keep alert id %d, got %d", created.ID, again.ID)
	}

	items, total, err := m.List(ctx, model.AlertFilter{Page: 1, PageSize: 20, Level: "critical"})
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Message != "node offline again" {
		t.Fatalf("unexpected alerts: total=%d %+v", total, items)
	}

	got, err := m.Get(ctx, created.ID)
	if err != nil || got.ObjectID != "n1" {
		t.Fatalf("get alert: %+v err=%v", got, err)
	}

	if err := m.InsertNotification(ctx, model.AlertNotification{AlertID: created.ID, ChannelID: 7, Status: "sent", ErrorCode: "", SentAt: now}); err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	ns, err := m.Notifications(ctx, created.ID)
	if err != nil || len(ns) != 1 || ns[0].ChannelID != 7 {
		t.Fatalf("notifications: %+v err=%v", ns, err)
	}

	if err := m.SetStatus(ctx, created.ID, model.AlertStatusResolved, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("set status: %v", err)
	}
	got, _ = m.Get(ctx, created.ID)
	if got.Status != model.AlertStatusResolved || got.ResolvedAt == nil {
		t.Fatalf("alert not resolved: %+v", got)
	}
}

func TestTrafficModel(t *testing.T) {
	db := newDB(t)
	m := model.NewTrafficModel(testsupport.Conn(db))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute)

	if err := m.UpsertSample(ctx, model.TrafficSample{
		EntityType: "node", EntityID: "n1", TS: now,
		UpBytes: 100, DownBytes: 200, Connections: 3,
	}); err != nil {
		t.Fatalf("upsert sample: %v", err)
	}

	points, err := m.Series(ctx, "node", "n1", "net", now.Add(-time.Hour), now.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(points) != 1 || points[0].UpBytes != 100 {
		t.Fatalf("unexpected series: %+v", points)
	}

	rows, total, err := m.Rows(ctx, "node", now.Add(-time.Hour), now.Add(time.Minute), 1, 20, "")
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if total != 1 || rows[0].UpBytes != 100 || rows[0].DownBytes != 200 {
		t.Fatalf("unexpected rows: total=%d %+v", total, rows)
	}
}

func TestClientModel(t *testing.T) {
	db := newDB(t)
	m := model.NewClientModel(testsupport.Conn(db))
	ctx := context.Background()
	now := time.Now().UTC()

	if err := m.Upsert(ctx, model.ClientRow{
		ClientID: "c1", Email: "a@example.com", PortID: "p1", PortGroupID: "pg1",
		UpBytes: 10, DownBytes: 20, LastOnlineAt: &now,
	}); err != nil {
		t.Fatalf("upsert client: %v", err)
	}
	items, total, err := m.List(ctx, model.ClientFilter{Page: 1, PageSize: 20, PortGroupID: "pg1"})
	if err != nil {
		t.Fatalf("list clients: %v", err)
	}
	if total != 1 || items[0].Email != "a@example.com" {
		t.Fatalf("unexpected clients: total=%d %+v", total, items)
	}
}

func TestSettingModel(t *testing.T) {
	db := newDB(t)
	m := model.NewSettingModel(testsupport.Conn(db))
	ctx := context.Background()

	if err := m.SetJSON(ctx, "probe", []byte(`{"frequencySeconds":30}`)); err != nil {
		t.Fatalf("set json: %v", err)
	}
	raw, found, err := m.GetJSON(ctx, "probe")
	if err != nil || !found {
		t.Fatalf("get json: found=%v err=%v", found, err)
	}
	if string(raw) != `{"frequencySeconds": 30}` && string(raw) != `{"frequencySeconds":30}` {
		t.Fatalf("unexpected json: %s", raw)
	}
	if _, found, err := m.GetJSON(ctx, "missing"); err != nil || found {
		t.Fatalf("missing json: found=%v err=%v", found, err)
	}
}

func TestSubscriptionModel(t *testing.T) {
	db := newDB(t)
	m := model.NewSubscriptionModel(testsupport.Conn(db))
	ctx := context.Background()

	h, err := m.CreateHost(ctx, model.SubscriptionHost{Host: "sub.example.com", EntryGroupID: "eg1", TLSMode: "panel"})
	if err != nil {
		t.Fatalf("create host: %v", err)
	}
	if h.ID == 0 {
		t.Fatal("host id is zero")
	}
	if _, err := m.CreateHost(ctx, model.SubscriptionHost{Host: "sub.example.com", EntryGroupID: "eg1", TLSMode: "panel"}); err == nil {
		t.Fatal("duplicate host should fail")
	}
	entry := "eg2"
	updated, err := m.UpdateHost(ctx, h.ID, &entry, nil)
	if err != nil || updated.EntryGroupID != "eg2" {
		t.Fatalf("update host: %+v err=%v", updated, err)
	}
	list, total, err := m.ListHosts(ctx, 1, 20)
	if err != nil || total != 1 || list[0].Host != "sub.example.com" {
		t.Fatalf("list hosts: total=%d %+v err=%v", total, list, err)
	}
	if err := m.DeleteHost(ctx, h.ID); err != nil {
		t.Fatalf("delete host: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO subscription_formats (key, label, enabled, sort_order) VALUES ('v2ray', 'V2Ray', true, 1)`); err != nil {
		t.Fatalf("seed format: %v", err)
	}
	formats, err := m.ListFormats(ctx)
	if err != nil || len(formats) != 1 {
		t.Fatalf("list formats: %+v err=%v", formats, err)
	}
	if err := m.SetFormatEnabled(ctx, "v2ray", false, 2); err != nil {
		t.Fatalf("set format: %v", err)
	}
	formats, _ = m.ListFormats(ctx)
	if formats[0].Enabled || formats[0].SortOrder != 2 {
		t.Fatalf("format not updated: %+v", formats[0])
	}
}

func TestNotificationModel(t *testing.T) {
	db := newDB(t)
	m := model.NewNotificationModel(testsupport.Conn(db))
	ctx := context.Background()

	c, err := m.Create(ctx, model.NotificationChannel{
		Name: "ops", Type: "webhook", TargetEncrypted: []byte("secret"), TargetMasked: "https://••••",
		MinLevel: "warning", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	updated, err := m.Update(ctx, c.ID, model.NotificationPatch{Enabled: boolPtr(false)})
	if err != nil || updated.Enabled {
		t.Fatalf("update channel: %+v err=%v", updated, err)
	}
	list, err := m.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list channels: %+v err=%v", list, err)
	}
	if err := m.Delete(ctx, c.ID); err != nil {
		t.Fatalf("delete channel: %v", err)
	}
	if _, err := m.Get(ctx, c.ID); err != model.ErrNotFound {
		t.Fatalf("get deleted = %v, want ErrNotFound", err)
	}
}

func TestGeoModel(t *testing.T) {
	db := newDB(t)
	m := model.NewGeoModel(testsupport.Conn(db))
	ctx := context.Background()

	f, err := m.Upsert(ctx, model.GeoFile{Kind: "geoip", Name: "geoip.dat", Version: "20261123", SizeBytes: 1024, Checksum: "abc", Status: "ready", Path: "/tmp/geoip.dat"})
	if err != nil {
		t.Fatalf("upsert geo: %v", err)
	}
	if err := m.ReplaceCategories(ctx, f.ID, []model.GeoCategory{{Category: "cn", EntriesCount: 10}}); err != nil {
		t.Fatalf("replace categories: %v", err)
	}
	files, total, err := m.List(ctx, "geoip", 1, 20)
	if err != nil || total != 1 || files[0].CategoriesCount != 1 {
		t.Fatalf("list geo: total=%d %+v err=%v", total, files, err)
	}
	cats, _, err := m.Categories(ctx, f.ID, 1, 50)
	if err != nil || len(cats) != 1 || cats[0].Category != "cn" {
		t.Fatalf("categories: %+v err=%v", cats, err)
	}
}

func TestAuditLogModel(t *testing.T) {
	db := newDB(t)
	m := model.NewAuditLogModel(testsupport.Conn(db))
	ctx := context.Background()
	now := time.Now().UTC()

	if err := m.Insert(ctx, model.AuditLog{
		Actor: "admin", Action: "settings.update", ObjectType: "settings", ObjectID: "probe",
		Result: "success", RequestID: "req-1", IP: "127.0.0.1", CreatedAt: now,
	}); err != nil {
		t.Fatalf("insert audit: %v", err)
	}
	items, total, err := m.List(ctx, model.AuditFilter{Page: 1, PageSize: 20, Action: "settings.update"})
	if err != nil || total != 1 || items[0].RequestID != "req-1" {
		t.Fatalf("list audit: total=%d %+v err=%v", total, items, err)
	}
	got, err := m.Get(ctx, items[0].ID)
	if err != nil || got.ObjectType != "settings" {
		t.Fatalf("get audit: %+v err=%v", got, err)
	}
}

func TestUpgradeModel(t *testing.T) {
	db := newDB(t)
	m := model.NewUpgradeModel(testsupport.Conn(db))
	ctx := context.Background()
	now := time.Now().UTC()
	taskID := "11111111-1111-1111-1111-111111111111"

	if err := m.CreateTask(ctx, &model.UpgradeTask{
		ID: taskID, Scope: "agent_batch", TargetVersion: "1.4.0", FromVersion: "1.3.0",
		Status: model.UpgradeTaskQueued, CreatedBy: "admin", RequestID: "req-1", CreatedAt: now,
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := m.UpsertTaskNode(ctx, &model.UpgradeTaskNode{
		TaskID: taskID, NodeID: "n1", FromVersion: "1.3.0", ToVersion: "1.4.0",
		State: model.NodeUpgradePending, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	got, err := m.GetTask(ctx, taskID)
	if err != nil || got.Scope != "agent_batch" || len(got.Nodes) != 1 {
		t.Fatalf("get task: %+v err=%v", got, err)
	}
	if err := m.UpdateTaskStatus(ctx, taskID, model.UpgradeTaskRunning, "", "", &now, nil); err != nil {
		t.Fatalf("update status: %v", err)
	}
	active, err := m.ActiveTask(ctx)
	if err != nil || active == nil || active.ID != taskID {
		t.Fatalf("active task: %+v err=%v", active, err)
	}

	ok, err := m.AcquireLock(ctx, "panel/replica-1", "agent_batch", now, 2*time.Hour)
	if err != nil || !ok {
		t.Fatalf("acquire lock: ok=%v err=%v", ok, err)
	}
	if ok, _ := m.AcquireLock(ctx, "other", "panel", now, time.Hour); ok {
		t.Fatal("second lock acquisition should fail")
	}
	if ok, err := m.RenewLock(ctx, "panel/replica-1", now.Add(time.Minute), 2*time.Hour); err != nil || !ok {
		t.Fatalf("renew lock: ok=%v err=%v", ok, err)
	}
	if err := m.ReleaseLock(ctx, "panel/replica-1"); err != nil {
		t.Fatalf("release lock: %v", err)
	}
	if lock, err := m.GetLock(ctx); err != nil || lock != nil {
		t.Fatalf("lock after release: %+v err=%v", lock, err)
	}

	if err := m.UpsertTarget(ctx, model.UpgradeTarget{NodeID: "n2", TargetVersion: "1.4.0", BatchID: taskID, CreatedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}
	targets, err := m.ListTargets(ctx)
	if err != nil || len(targets) != 1 {
		t.Fatalf("targets: %+v err=%v", targets, err)
	}
	if err := m.DeleteTargetsByBatch(ctx, taskID); err != nil {
		t.Fatalf("delete targets: %v", err)
	}
}

func boolPtr(v bool) *bool { return &v }
