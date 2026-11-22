package dispatch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/relayx-tech/panel/internal/cryptobox"
	"github.com/relayx-tech/panel/internal/logic/dispatch"
	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func setupDB(t *testing.T) sqlx.SqlConn {
	t.Helper()
	db := testsupport.NewPostgres(t)
	if err := migrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return sqlx.NewSqlConnFromDB(db)
}

func TestSQLStore(t *testing.T) {
	box, err := cryptobox.New(make([]byte, 32), "k1")
	if err != nil {
		t.Fatal(err)
	}
	svc := dispatch.New(dispatch.NewSQLStore(setupDB(t)), box)
	ctx := context.Background()

	op1, err := svc.BeginOperation(ctx, dispatch.Operation{
		OperationID: uuid.NewString(), NodeID: "node-1", Type: "apply", RequestID: "rid",
	})
	if err != nil || op1.Version != 1 {
		t.Fatalf("op1 = %+v, err = %v", op1, err)
	}
	op2, err := svc.BeginOperation(ctx, dispatch.Operation{
		OperationID: uuid.NewString(), NodeID: "node-1", Type: "apply",
	})
	if err != nil || op2.Version != 2 {
		t.Fatalf("op2 = %+v, err = %v", op2, err)
	}
	if err := svc.Transition(ctx, op1.OperationID, dispatch.StateApplied, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordSnapshot(ctx, "node-1", op1.Version, []byte(`{"v":1}`), "h", "test"); err != nil {
		t.Fatal(err)
	}
}

func TestSQLAuditSink(t *testing.T) {
	sink := dispatch.NewSQLAuditSink(setupDB(t))
	if err := sink.Record(context.Background(), dispatch.AuditEntry{
		Actor: "panel", Action: "config.dispatch.apply", Object: "node-1", RequestID: "rid", Result: "success",
	}); err != nil {
		t.Fatal(err)
	}
}
