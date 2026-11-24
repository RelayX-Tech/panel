package migrate_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
)

// design.md §6.2 迁移清单（9 个文件，按版本升序）。
var sto004Migrations = []string{
	"202611231200_create_alerts.sql",
	"202611231210_create_node_events.sql",
	"202611231220_create_client_stats.sql",
	"202611231230_create_traffic_partitions.sql",
	"202611231240_create_subscription_settings.sql",
	"202611231250_create_geo_files.sql",
	"202611231260_create_settings_apply.sql",
	"202611231270_create_upgrade.sql",
	"202611231280_extend_audit_logs.sql",
	"202611231290_add_auth_users_role.sql",
}

// 19 张新表（design.md §6.2）。
var sto004Tables = []string{
	"alerts",
	"notification_channels",
	"alert_notifications",
	"node_events",
	"client_stats",
	"traffic_samples",
	"traffic_1m",
	"traffic_1h",
	"subscription_hosts",
	"subscription_formats",
	"geo_files",
	"geo_file_categories",
	"panel_settings",
	"settings_apply_tasks",
	"settings_apply_task_nodes",
	"upgrade_tasks",
	"upgrade_task_nodes",
	"node_upgrade_targets",
	"upgrade_lock",
}

func TestSTO004MigrationFiles(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var got []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "20261123") {
			got = append(got, e.Name())
		}
	}
	sort.Strings(got)
	if len(got) != len(sto004Migrations) {
		t.Fatalf("migration count = %d (%v), want %d", len(got), got, len(sto004Migrations))
	}
	for i, name := range sto004Migrations {
		if got[i] != name {
			t.Errorf("migration[%d] = %s, want %s", i, got[i], name)
		}
	}
}

func TestSTO004MigrationsUpDown(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	if err := migrate.Up(ctx, db, migrations.FS); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	for _, table := range sto004Tables {
		if !tableExists(t, db, table) {
			t.Errorf("table %s missing after up", table)
		}
	}

	// audit_logs 扩展列（design.md §6.2）。
	for _, col := range []string{"object_type", "object_id", "ip"} {
		if !columnExists(t, db, "audit_logs", col) {
			t.Errorf("audit_logs.%s missing after up", col)
		}
	}

	// 流量分区：当月至少一个分区，且分区父表已建。
	if n := countPartitions(t, db); n < 3 {
		t.Errorf("traffic partitions = %d, want at least 3 (samples/1m/1h current month)", n)
	}

	// Down 9 次回到 STO-004 之前的 schema。
	for range sto004Migrations {
		if err := migrate.Down(ctx, db, migrations.FS); err != nil {
			t.Fatalf("migrate down: %v", err)
		}
	}
	for _, table := range sto004Tables {
		if tableExists(t, db, table) {
			t.Errorf("table %s still exists after down", table)
		}
	}
	if !tableExists(t, db, "auth_users") {
		t.Error("auth_users should survive STO-004 rollback")
	}
	if columnExists(t, db, "audit_logs", "object_type") {
		t.Error("audit_logs.object_type should be removed after down")
	}
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRowContext(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)`,
		table).Scan(&exists)
	if err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	return exists
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRowContext(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2)`,
		table, column).Scan(&exists)
	if err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	return exists
}

func countPartitions(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM pg_class c
		 JOIN pg_inherits i ON i.inhrelid = c.oid
		 JOIN pg_class p ON p.oid = i.inhparent
		 WHERE p.relname IN ('traffic_samples','traffic_1m','traffic_1h')`).Scan(&n)
	if err != nil {
		t.Fatalf("count partitions: %v", err)
	}
	return n
}
