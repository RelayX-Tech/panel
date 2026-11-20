package model

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
)

func migrateUp(ctx context.Context, db *sql.DB) error {
	return migrate.Up(ctx, db, migrations.FS)
}

func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	testsupport.RequireRedis(t)
	db := testsupport.NewPostgres(t)
	if err := migrateUp(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newModels 构造用户 / 会话适配层（共享连接与 sqlcache 配置）。
func newModels(t *testing.T) (*PostgresAuthUserModel, *PostgresAuthSessionModel) {
	t.Helper()
	conn := testsupport.Conn(migratedDB(t))
	users := NewAuthUserModel(conn, testsupport.CacheConf())
	return users, NewAuthSessionModel(conn, testsupport.CacheConf(), users)
}

func TestUserCreateAndFindCaseInsensitive(t *testing.T) {
	users, _ := newModels(t)
	m := users
	ctx := context.Background()

	u, err := m.Create(ctx, nil, "Admin", "$argon2id$dummy", "Administrator")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == "" || u.Account != "Admin" {
		t.Fatalf("created user = %+v", u)
	}

	found, err := m.FindByAccount(ctx, "admin")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.ID != u.ID || found.PasswordHash != "$argon2id$dummy" || found.DisplayName != "Administrator" {
		t.Errorf("found user = %+v", found)
	}
	if found.DisabledAt != nil {
		t.Errorf("DisabledAt = %v, want nil", found.DisabledAt)
	}

	if _, err := m.FindByAccount(ctx, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing user error = %v, want ErrNotFound", err)
	}
}

func TestUserAccountUniqueCaseInsensitive(t *testing.T) {
	users, _ := newModels(t)
	m := users
	ctx := context.Background()

	if _, err := m.Create(ctx, nil, "admin", "hash1", ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.Create(ctx, nil, "ADMIN", "hash2", ""); err == nil {
		t.Fatal("duplicate account with different case was accepted")
	}
}

func TestUserCount(t *testing.T) {
	users, _ := newModels(t)
	m := users
	ctx := context.Background()

	n, err := m.Count(ctx, nil)
	if err != nil || n != 0 {
		t.Fatalf("count = %d, %v; want 0", n, err)
	}
	if _, err := m.Create(ctx, nil, "admin", "hash", ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if n, _ := m.Count(ctx, nil); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

func TestSessionLifecycle(t *testing.T) {
	users, sessions := newModels(t)
	ctx := context.Background()

	u, err := users.Create(ctx, nil, "admin", "hash", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	s := &Session{
		SessionHash:       hash,
		UserID:            u.ID,
		CreatedAt:         now,
		LastSeenAt:        now,
		ExpiresAt:         now.Add(24 * time.Hour),
		AbsoluteExpiresAt: now.Add(168 * time.Hour),
		ClientIP:          "127.0.0.1",
		UserAgent:         "test-agent",
	}
	if err := sessions.Create(ctx, nil, s); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := sessions.FindByHash(ctx, hash)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if got.Session.ID != s.ID || got.User.ID != u.ID {
		t.Fatalf("session user mismatch: %+v", got)
	}
	if got.Session.ClientIP != "127.0.0.1" || got.Session.UserAgent != "test-agent" {
		t.Errorf("client metadata = %+v", got.Session)
	}
	if !got.Session.ExpiresAt.Equal(now.Add(24 * time.Hour)) {
		t.Errorf("expiresAt = %v", got.Session.ExpiresAt)
	}

	newExpiry := now.Add(48 * time.Hour)
	if err := sessions.Touch(ctx, nil, s.ID, now.Add(time.Minute), newExpiry); err != nil {
		t.Fatalf("touch: %v", err)
	}
	got, _ = sessions.FindByHash(ctx, hash)
	if !got.Session.ExpiresAt.Equal(newExpiry) {
		t.Errorf("expiresAt after touch = %v, want %v", got.Session.ExpiresAt, newExpiry)
	}
	if !got.Session.LastSeenAt.Equal(now.Add(time.Minute)) {
		t.Errorf("lastSeenAt = %v", got.Session.LastSeenAt)
	}

	if err := sessions.RevokeByHash(ctx, nil, hash); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	got, _ = sessions.FindByHash(ctx, hash)
	if got.Session.RevokedAt == nil {
		t.Error("revokedAt not set after revoke")
	}
}

func TestSessionFindMissing(t *testing.T) {
	_, sessions := newModels(t)
	if _, err := sessions.FindByHash(context.Background(), []byte("not-a-session")); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	users, sessions := newModels(t)
	ctx := context.Background()

	u, _ := users.Create(ctx, nil, "admin", "hash", "")
	now := time.Now().UTC()

	mk := func(hash byte, expires time.Time) {
		s := &Session{
			SessionHash:       []byte{hash},
			UserID:            u.ID,
			CreatedAt:         now,
			LastSeenAt:        now,
			ExpiresAt:         expires,
			AbsoluteExpiresAt: now.Add(168 * time.Hour),
		}
		if err := sessions.Create(ctx, nil, s); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}
	mk(1, now.Add(-8*24*time.Hour)) // 超期 7 天以上，应删除
	mk(2, now.Add(-time.Hour))      // 已过期但保留窗口内

	deleted, err := sessions.DeleteExpiredBefore(ctx, nil, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
	if _, err := sessions.FindByHash(ctx, []byte{2}); err != nil {
		t.Errorf("recent expired session was deleted: %v", err)
	}
	if _, err := sessions.FindByHash(ctx, []byte{1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("old session still present: %v", err)
	}
}
