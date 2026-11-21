package migrate

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"
	"time"

	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
)

func goodFS() fstest.MapFS {
	return fstest.MapFS{
		"202601010001_create_a.sql": &fstest.MapFile{Data: []byte(`
-- +goose Up
CREATE TABLE a (id int primary key);
-- +goose Down
DROP TABLE a;
`)},
		"202601010002_create_b.sql": &fstest.MapFile{Data: []byte(`
-- +goose Up
CREATE TABLE b (id int primary key);
-- +goose Down
DROP TABLE b;
`)},
	}
}

func migrationCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version_id > 0 AND is_applied`).Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	return n
}

func TestUpIsIdempotent(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	if err := Up(ctx, db, goodFS()); err != nil {
		t.Fatalf("first up: %v", err)
	}
	if n := migrationCount(t, db); n != 2 {
		t.Fatalf("migrations recorded = %d, want 2", n)
	}

	if err := Up(ctx, db, goodFS()); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if n := migrationCount(t, db); n != 2 {
		t.Fatalf("migrations recorded after rerun = %d, want 2", n)
	}
}

func TestUpDoesNotRecordFailedMigration(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	bad := fstest.MapFS{
		"202601010001_create_a.sql": &fstest.MapFile{Data: []byte(`
-- +goose Up
CREATE TABLE a (id int primary key);
`)},
		"202601010002_broken.sql": &fstest.MapFile{Data: []byte(`
-- +goose Up
CREATE TABLE (;
`)},
	}
	if err := Up(ctx, db, bad); err == nil {
		t.Fatal("expected migration failure")
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("check migrations table: %v", err)
	}
	if !exists {
		return
	}
	var applied int64
	err := db.QueryRow(`SELECT version_id FROM schema_migrations WHERE version_id > 0 AND is_applied ORDER BY version_id DESC LIMIT 1`).Scan(&applied)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("query migrations: %v", err)
	}
	if applied == 202601010002 {
		t.Fatal("failed migration was recorded")
	}
}

func TestUpSerializesConcurrentRunners(t *testing.T) {
	db := testsupport.NewPostgres(t)
	fsys := goodFS()

	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			errs <- Up(ctx, db, fsys)
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent up: %v", err)
		}
	}
	if n := migrationCount(t, db); n != 2 {
		t.Fatalf("migrations recorded = %d, want 2", n)
	}
}

func TestUpRejectsMigrationWithoutUpSection(t *testing.T) {
	db := testsupport.NewPostgres(t)
	bad := fstest.MapFS{
		"202601010001_empty.sql": &fstest.MapFile{Data: []byte("-- +goose Down\nDROP TABLE a;")},
	}
	if err := Up(context.Background(), db, bad); err == nil {
		t.Fatal("migration without up section should be rejected")
	}

	empty := fstest.MapFS{
		"202601010002_empty.sql": &fstest.MapFile{Data: []byte("-- +goose Up\n-- +goose Down\n")},
	}
	if err := Up(context.Background(), db, empty); err == nil {
		t.Fatal("empty up section should be rejected")
	}
}

func TestUpRejectsBadFilename(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	bad := fstest.MapFS{
		"create_a.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE a (id int);")},
	}
	if err := Up(ctx, db, bad); err == nil {
		t.Fatal("migration with non-conforming filename should be rejected")
	}
}

func TestCheckRejectsPendingMigrations(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	if err := Check(ctx, db, goodFS()); err == nil {
		t.Fatal("Check should reject pending migrations")
	}
	if err := Up(ctx, db, goodFS()); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := Check(ctx, db, goodFS()); err != nil {
		t.Fatalf("Check after up: %v", err)
	}
}

func TestDownRollsBackLatest(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	if err := Up(ctx, db, goodFS()); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := Down(ctx, db, goodFS()); err != nil {
		t.Fatalf("down: %v", err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('b') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("check table b: %v", err)
	}
	if exists {
		t.Fatal("table b still exists after down")
	}
}

func TestVersionReturnsLatestApplied(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	if err := Up(ctx, db, goodFS()); err != nil {
		t.Fatalf("up: %v", err)
	}
	version, err := Version(ctx, db, goodFS())
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != 202601010002 {
		t.Fatalf("version = %d, want 202601010002", version)
	}
}

func TestReadyCheckReflectsSchemaState(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	if err := ReadyCheck(db, goodFS())(); err == nil {
		t.Fatal("ready check should fail with pending migrations")
	}
	if err := Up(ctx, db, goodFS()); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := ReadyCheck(db, goodFS())(); err != nil {
		t.Fatalf("ready check after up: %v", err)
	}
}

func TestUpRejectsMissingDownMarker(t *testing.T) {
	db := testsupport.NewPostgres(t)
	bad := fstest.MapFS{
		"202601010001_x.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE x (id int);")},
	}
	if err := Up(context.Background(), db, bad); err == nil {
		t.Fatal("migration without down marker should be rejected")
	}
}

func TestDownWithoutMigrations(t *testing.T) {
	db := testsupport.NewPostgres(t)
	if err := Down(context.Background(), db, goodFS()); err == nil {
		t.Fatal("down without applied migrations should fail")
	}
}

// TC-BE-045/046：真实迁移创建 auth_password_audits 与索引，Down 可回滚。
func TestPasswordAuditMigrationUpDown(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()

	if err := Up(ctx, db, migrations.FS); err != nil {
		t.Fatalf("up: %v", err)
	}
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('public.auth_password_audits') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("check audit table: %v", err)
	}
	if !exists {
		t.Fatal("auth_password_audits not created")
	}

	statuses, err := Status(ctx, db, migrations.FS)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, s := range statuses {
		if s.State != "applied" {
			t.Fatalf("migration %v state = %v", s.Source, s.State)
		}
	}

	if err := Down(ctx, db, migrations.FS); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := db.QueryRow(`SELECT to_regclass('public.auth_password_audits') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("check audit table after down: %v", err)
	}
	if exists {
		t.Fatal("auth_password_audits still exists after down")
	}
}
