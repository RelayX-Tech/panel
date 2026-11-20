package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/relayx-tech/panel/internal/migrate"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/testsupport"
	"github.com/relayx-tech/panel/migrations"
)

// txFixtures 构造真实 PostgreSQL + 适配层；事务边界在调用方（logic 层）。
func txFixtures(t *testing.T) (sqlx.SqlConn, *model.PostgresAuthUserModel, *model.PostgresAuthSessionModel) {
	t.Helper()
	testsupport.RequireRedis(t)
	db := testsupport.NewPostgres(t)
	if err := migrate.Up(context.Background(), db, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	conn := testsupport.Conn(db)
	users := model.NewAuthUserModel(conn, testsupport.CacheConf())
	sessions := model.NewAuthSessionModel(conn, testsupport.CacheConf(), users)
	return conn, users, sessions
}

func newTestSession(userID, hash string, now time.Time) *model.Session {
	return &model.Session{
		SessionHash:       []byte(hash),
		UserID:            userID,
		CreatedAt:         now,
		LastSeenAt:        now,
		ExpiresAt:         now.Add(time.Hour),
		AbsoluteExpiresAt: now.Add(2 * time.Hour),
	}
}

// TC-BE-008：同一 TransactCtx 内两次写成功提交。
func TestTransactCtxCommitsBothWrites(t *testing.T) {
	conn, users, sessions := txFixtures(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	err := conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		u, err := users.Create(ctx, session, "alice", "hash", "Alice")
		if err != nil {
			return err
		}
		return sessions.Create(ctx, session, newTestSession(u.ID, "hash-commit", now))
	})
	if err != nil {
		t.Fatalf("transact: %v", err)
	}

	if _, err := users.FindByAccount(ctx, "alice"); err != nil {
		t.Errorf("user not committed: %v", err)
	}
	if _, err := sessions.FindByHash(ctx, []byte("hash-commit")); err != nil {
		t.Errorf("session not committed: %v", err)
	}
}

// TC-BE-009：事务内第二次写失败 → 第一次写入整体回滚。
func TestTransactCtxRollsBackOnSecondFailure(t *testing.T) {
	conn, users, sessions := txFixtures(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	seed, err := users.Create(ctx, nil, "seed", "hash", "Seed")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// 预置冲突行，制造 session_hash 唯一键冲突。
	if err := sessions.Create(ctx, nil, newTestSession(seed.ID, "dup-hash", now)); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	err = conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		u, err := users.Create(ctx, session, "bob", "hash", "Bob")
		if err != nil {
			return err
		}
		return sessions.Create(ctx, session, newTestSession(u.ID, "dup-hash", now))
	})
	if err == nil {
		t.Fatal("expected unique violation, got nil")
	}

	// 第一次写入不得落库。
	if _, err := users.FindByAccount(ctx, "bob"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("user bob persisted after rollback: err=%v", err)
	}
	// 预置数据不受影响。
	if _, err := users.FindByAccount(ctx, "seed"); err != nil {
		t.Errorf("seed user lost: %v", err)
	}
}
