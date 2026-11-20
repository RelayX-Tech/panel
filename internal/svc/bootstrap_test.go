package svc

import (
	"context"
	"database/sql"
	"testing"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
)

func bootstrapDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testsupport.NewPostgres(t)
	if err := migrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestBootstrapAdminCreatesOnce(t *testing.T) {
	db := bootstrapDB(t)
	users := model.NewAuthUserModel(db)
	ctx := context.Background()

	res, err := BootstrapAdmin(ctx, db, users, "admin", "sup3r-secret-pass")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if !res.Created || res.Account != "admin" {
		t.Fatalf("result = %+v", res)
	}

	u, err := users.FindByAccount(ctx, "admin")
	if err != nil {
		t.Fatalf("find admin: %v", err)
	}
	ok, err := auth.VerifyPassword("sup3r-secret-pass", u.PasswordHash)
	if err != nil || !ok {
		t.Fatalf("admin password hash does not verify: ok=%v err=%v", ok, err)
	}

	// 再次启动不重复创建
	res, err = BootstrapAdmin(ctx, db, users, "admin", "sup3r-secret-pass")
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if res.Created {
		t.Error("admin was created twice")
	}
	if n, _ := users.Count(ctx); n != 1 {
		t.Errorf("user count = %d, want 1", n)
	}
}

func TestBootstrapAdminRejectsShortPassword(t *testing.T) {
	db := bootstrapDB(t)
	users := model.NewAuthUserModel(db)

	res, err := BootstrapAdmin(context.Background(), db, users, "admin", "short-pass1")
	if err == nil {
		t.Fatal("expected error for password shorter than 12 chars")
	}
	if res.Created {
		t.Error("admin created with short password")
	}
	if n, _ := users.Count(context.Background()); n != 0 {
		t.Errorf("user count = %d, want 0", n)
	}
}

func TestBootstrapAdminSkipsWhenUsersExist(t *testing.T) {
	db := bootstrapDB(t)
	users := model.NewAuthUserModel(db)

	if _, err := BootstrapAdmin(context.Background(), db, users, "admin", "sup3r-secret-pass"); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	res, err := BootstrapAdmin(context.Background(), db, users, "second", "another-secret-pass")
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if res.Created {
		t.Error("second admin created although users exist")
	}
	if _, err := users.FindByAccount(context.Background(), "second"); err == nil {
		t.Error("second account should not exist")
	}
}

func TestBootstrapAdminMissingEnv(t *testing.T) {
	db := bootstrapDB(t)
	users := model.NewAuthUserModel(db)

	t.Setenv("RELAYX_ADMIN_ACCOUNT", "")
	t.Setenv("RELAYX_ADMIN_PASSWORD", "")

	res, err := BootstrapAdminFromEnv(context.Background(), db, users)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if res.Created {
		t.Error("admin created without env credentials")
	}
	if res.Warning == "" {
		t.Error("missing env should produce a warning")
	}
}

func TestBootstrapAdminConcurrent(t *testing.T) {
	db := bootstrapDB(t)
	users := model.NewAuthUserModel(db)

	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := BootstrapAdmin(context.Background(), db, users, "admin", "sup3r-secret-pass")
			errs <- err
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent bootstrap: %v", err)
		}
	}
	if n, _ := users.Count(context.Background()); n != 1 {
		t.Errorf("user count = %d, want 1", n)
	}
}
