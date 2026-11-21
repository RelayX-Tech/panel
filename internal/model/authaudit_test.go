package model

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/testsupport"
)

func auditFixtures(t *testing.T) (*sql.DB, *PostgresAuthUserModel, *PostgresAuthPasswordAuditModel) {
	t.Helper()
	db := migratedDB(t)
	conn := testsupport.Conn(db)
	return db, NewAuthUserModel(conn, testsupport.CacheConf()), NewAuthPasswordAuditModel(conn)
}

// TC-BE-037/039：成功修改写入完整审计字段，且可按 user_id / request_id 检索。
func TestPasswordAuditInsertAndQuery(t *testing.T) {
	_, users, audits := auditFixtures(t)
	ctx := context.Background()

	u, err := users.Create(ctx, nil, "audit-user", "hash", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	audit := &PasswordAudit{
		UserID:    u.ID,
		RequestID: "01J8Z6YQ9K3N4R5T6V7W8X9Y0Z",
		ClientIP:  "203.0.113.7",
		Result:    "success",
	}
	if err := audits.Insert(ctx, nil, audit); err != nil {
		t.Fatalf("insert audit: %v", err)
	}

	got, err := audits.ListByUserID(ctx, u.ID, 10)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("audits = %d, want 1", len(got))
	}
	row := got[0]
	if row.ID == "" || row.UserID != u.ID || row.RequestID != audit.RequestID {
		t.Errorf("audit row = %+v", row)
	}
	if row.ClientIP != "203.0.113.7" || row.Result != "success" || row.OccurredAt.IsZero() {
		t.Errorf("audit row = %+v", row)
	}

	byRequest, err := audits.FindByRequestID(ctx, audit.RequestID)
	if err != nil {
		t.Fatalf("find by request: %v", err)
	}
	if len(byRequest) != 1 || byRequest[0].ID != row.ID {
		t.Errorf("by request = %+v", byRequest)
	}
}

// TC-BE-038：审计表不包含密码 / 哈希 / Cookie / Token 列。
func TestPasswordAuditTableHasNoSecrets(t *testing.T) {
	db, _, _ := auditFixtures(t)

	var n int
	err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = 'auth_password_audits'
  AND column_name IN ('password', 'password_hash', 'cookie', 'token', 'request_body')`).Scan(&n)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	if n != 0 {
		t.Fatalf("auth_password_audits has %d secret-like column(s)", n)
	}
}

// TC-BE-042：清理任务只删除 occurred_at 早于保留期的行。
func TestDeleteBefore(t *testing.T) {
	db, users, audits := auditFixtures(t)
	ctx := context.Background()

	u, err := users.Create(ctx, nil, "audit-cleanup", "hash", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO auth_password_audits (user_id, request_id, client_ip, result, occurred_at)
VALUES ($1::uuid, 'old', '203.0.113.7', 'success', now() - interval '181 days'),
       ($1::uuid, 'new', '203.0.113.7', 'success', now())`, u.ID); err != nil {
		t.Fatalf("seed audits: %v", err)
	}

	deleted, err := audits.DeleteBefore(ctx, nil, time.Now().UTC().Add(-180*24*time.Hour))
	if err != nil {
		t.Fatalf("delete before: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	remaining, err := audits.ListByUserID(ctx, u.ID, 10)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(remaining) != 1 || remaining[0].RequestID != "new" {
		t.Fatalf("remaining = %+v, want the recent row only", remaining)
	}
}
