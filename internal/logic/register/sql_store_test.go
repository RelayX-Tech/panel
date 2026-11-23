package register_test

import (
	"context"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/logic/register"
	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestSQLStoreTokenAndCert(t *testing.T) {
	db := testsupport.NewPostgres(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := register.NewSQLStore(sqlx.NewSqlConnFromDB(db))

	rec := register.TokenRecord{Hash: "hash-1", NodeType: "entry", ExpiresAt: time.Now().Add(24 * time.Hour)}
	if err := store.CreateToken(ctx, rec); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.GetToken(ctx, "hash-1")
	if err != nil || !ok || got.NodeType != "entry" || !got.UsedAt.IsZero() {
		t.Fatalf("GetToken = %+v, ok=%v, err=%v", got, ok, err)
	}
	if _, ok, _ := store.GetToken(ctx, "missing"); ok {
		t.Fatal("不存在的 Token 应返回 false")
	}

	if err := store.MarkTokenUsed(ctx, "hash-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	used, _, _ := store.GetToken(ctx, "hash-1")
	if used.UsedAt.IsZero() {
		t.Fatal("used_at 未更新")
	}

	if err := store.SaveCert(ctx, register.CertRecord{NodeID: "node-1", Fingerprint: "fp", NotAfter: time.Now().Add(90 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	cert, ok, err := store.GetCert(ctx, "node-1")
	if err != nil || !ok || cert.Fingerprint != "fp" || !cert.RevokedAt.IsZero() {
		t.Fatalf("GetCert = %+v, ok=%v, err=%v", cert, ok, err)
	}

	if err := store.RevokeCert(ctx, "node-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	cert, _, _ = store.GetCert(ctx, "node-1")
	if cert.RevokedAt.IsZero() {
		t.Fatal("revoked_at 未更新")
	}
}
