package model

import (
	"context"
	"database/sql"
	"testing"

	"github.com/relayx-tech/panel/internal/testsupport"
)

// cacheFixtures 构造共享连接的用户 / 会话适配层，并返回原始 *sql.DB 供绕过缓存的直写。
func cacheFixtures(t *testing.T) (*sql.DB, *PostgresAuthUserModel, *PostgresAuthSessionModel) {
	t.Helper()
	db := migratedDB(t)
	conn := testsupport.Conn(db)
	users := NewAuthUserModel(conn, testsupport.CacheConf())
	return db, users, NewAuthSessionModel(conn, testsupport.CacheConf(), users)
}

// TC-BE-004：用户主键点查命中 sqlcache；生成 Update 后读到新值；条件查询不缓存。
func TestFindOneUsesSqlcacheAndGeneratedUpdateInvalidates(t *testing.T) {
	db, users, _ := cacheFixtures(t)
	ctx := context.Background()

	u, err := users.Create(ctx, nil, "cache-user", "hash", "Old Name")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 创建路径已回读并写入缓存。
	got, err := users.FindOne(ctx, u.ID)
	if err != nil {
		t.Fatalf("find one: %v", err)
	}
	if got.DisplayName != "Old Name" {
		t.Fatalf("displayName = %q", got.DisplayName)
	}

	// 绕过缓存直写数据库：下一次点查仍命中旧缓存值，证明走的是 sqlcache。
	if _, err := db.Exec(`UPDATE auth_users SET display_name = 'Direct DB' WHERE id = $1::uuid`, u.ID); err != nil {
		t.Fatalf("direct update: %v", err)
	}
	cached, err := users.FindOne(ctx, u.ID)
	if err != nil {
		t.Fatalf("find one cached: %v", err)
	}
	if cached.DisplayName != "Old Name" {
		t.Fatalf("expected cache hit (Old Name), got %q", cached.DisplayName)
	}

	// 生成 Update 自动失效 sqlcache，之后点查读到新值。
	if err := users.generated.Update(ctx, &AuthUsers{
		Id:           u.ID,
		Account:      u.Account,
		PasswordHash: u.PasswordHash,
		DisplayName:  "Updated",
	}); err != nil {
		t.Fatalf("generated update: %v", err)
	}
	updated, err := users.FindOne(ctx, u.ID)
	if err != nil {
		t.Fatalf("find one after update: %v", err)
	}
	if updated.DisplayName != "Updated" {
		t.Errorf("displayName after update = %q, want Updated", updated.DisplayName)
	}

	// 条件查询不走缓存：直写后立即可见。
	if _, err := db.Exec(`UPDATE auth_users SET display_name = 'Cond New' WHERE id = $1::uuid`, u.ID); err != nil {
		t.Fatalf("direct update 2: %v", err)
	}
	cond, err := users.FindByAccount(ctx, "cache-user")
	if err != nil {
		t.Fatalf("find by account: %v", err)
	}
	if cond.DisplayName != "Cond New" {
		t.Errorf("conditional query used cache: %q", cond.DisplayName)
	}
}

// TC-BE-004（会话侧）：会话按 session_hash 查询不缓存，吊销立即可见。
func TestSessionQueriesAreNotCached(t *testing.T) {
	db, users, sessions := cacheFixtures(t)
	ctx := context.Background()

	u, err := users.Create(ctx, nil, "session-user", "hash", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	s := &Session{
		SessionHash:       []byte("session-hash"),
		UserID:            u.ID,
		ExpiresAt:         u.CreatedAt.Add(1),
		AbsoluteExpiresAt: u.CreatedAt.Add(2),
		LastSeenAt:        u.CreatedAt,
		CreatedAt:         u.CreatedAt,
	}
	if err := sessions.Create(ctx, nil, s); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := sessions.FindByHash(ctx, []byte("session-hash")); err != nil {
		t.Fatalf("find session: %v", err)
	}

	// 直写吊销：若会话被缓存，下一次查询仍会返回未吊销状态。
	if _, err := db.Exec(`UPDATE auth_sessions SET revoked_at = now() WHERE id = $1::uuid`, s.ID); err != nil {
		t.Fatalf("direct revoke: %v", err)
	}
	got, err := sessions.FindByHash(ctx, []byte("session-hash"))
	if err != nil {
		t.Fatalf("find session after revoke: %v", err)
	}
	if got.Session.RevokedAt == nil {
		t.Error("session query returned stale (cached) row")
	}
}
