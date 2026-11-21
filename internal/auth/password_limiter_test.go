package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/testsupport"
)

const testWindow = 15 * time.Minute

func newLimiter(t *testing.T, window time.Duration, maxAttempts int) (*auth.PasswordLimiter, *redis.Redis) {
	t.Helper()
	testsupport.RequireRedis(t)
	client := redis.RedisConf{Host: testsupport.RedisAddr(), Type: "node"}.NewRedis()
	limiter := auth.NewPasswordLimiter(client, window, maxAttempts)
	t.Cleanup(func() {
		_ = limiter.Reset(context.Background(), t.Name())
	})
	return limiter, client
}

// TC-BE-023：同一用户 15 分钟内旧密码错误 1~4 次均返回可重试（未锁定）。
func TestPasswordLimiter_FirstFourFailures(t *testing.T) {
	limiter, _ := newLimiter(t, testWindow, 5)
	ctx := context.Background()

	for i := 1; i <= 4; i++ {
		count, retryAfter, err := limiter.RecordFailure(ctx, t.Name())
		if err != nil {
			t.Fatalf("record failure %d: %v", i, err)
		}
		if count != i {
			t.Fatalf("count = %d, want %d", count, i)
		}
		if retryAfter <= 0 {
			t.Fatalf("retryAfter = %d, want > 0", retryAfter)
		}
		locked, _, err := limiter.Check(ctx, t.Name())
		if err != nil {
			t.Fatalf("check %d: %v", i, err)
		}
		if locked {
			t.Fatalf("locked after %d failures, want unlocked", i)
		}
	}
}

// TC-BE-024：第 5 次旧密码错误进入锁定，返回剩余秒数（≥1）。
func TestPasswordLimiter_FifthFailureLocked(t *testing.T) {
	limiter, _ := newLimiter(t, testWindow, 5)
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		if _, _, err := limiter.RecordFailure(ctx, t.Name()); err != nil {
			t.Fatalf("record failure %d: %v", i, err)
		}
	}
	locked, retryAfter, err := limiter.Check(ctx, t.Name())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !locked {
		t.Fatal("not locked after 5 failures")
	}
	if retryAfter < 1 {
		t.Fatalf("retryAfter = %d, want >= 1", retryAfter)
	}
}

// TC-BE-026：锁定窗口到期后自动解锁。
func TestPasswordLimiter_WindowExpires(t *testing.T) {
	limiter, _ := newLimiter(t, 300*time.Millisecond, 2)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, _, err := limiter.RecordFailure(ctx, t.Name()); err != nil {
			t.Fatalf("record failure %d: %v", i, err)
		}
	}
	if locked, _, _ := limiter.Check(ctx, t.Name()); !locked {
		t.Fatal("not locked after reaching threshold")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		locked, _, err := limiter.Check(ctx, t.Name())
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		if !locked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("limiter did not unlock after window expiry")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TC-BE-027：成功修改后计数清零（key 被删除）。
func TestPasswordLimiter_ResetOnSuccess(t *testing.T) {
	limiter, _ := newLimiter(t, testWindow, 5)
	ctx := context.Background()

	if _, _, err := limiter.RecordFailure(ctx, t.Name()); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	if err := limiter.Reset(ctx, t.Name()); err != nil {
		t.Fatalf("reset: %v", err)
	}

	count, _, err := limiter.RecordFailure(ctx, t.Name())
	if err != nil {
		t.Fatalf("record after reset: %v", err)
	}
	if count != 1 {
		t.Fatalf("count after reset = %d, want 1", count)
	}
}

// TC-BE-028：修改密码限流使用独立命名空间，不读写登录限流的 key。
func TestPasswordLimiter_IndependentFromLogin(t *testing.T) {
	limiter, client := newLimiter(t, testWindow, 5)
	ctx := context.Background()

	if _, _, err := limiter.RecordFailure(ctx, t.Name()); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	if value, err := client.GetCtx(ctx, auth.PasswordFailureKeyPrefix+t.Name()); err != nil || value != "1" {
		t.Fatalf("password key = %q, %v; want \"1\"", value, err)
	}
	// 登录限流（来源 IP 进程内）不写 relayx:auth:login:* 之外的 key；
	// 修改密码计数不得出现在登录命名空间。
	loginKey := "relayx:auth:login:" + t.Name()
	exists, err := client.ExistsCtx(ctx, loginKey)
	if err != nil {
		t.Fatalf("exists login key: %v", err)
	}
	if exists {
		t.Fatal("password limiter wrote to login namespace")
	}
}

// TC-BE-030：首次错误写入 900 秒 TTL；后续错误不重置 TTL。
func TestPasswordLimiter_TTL(t *testing.T) {
	limiter, client := newLimiter(t, testWindow, 5)
	ctx := context.Background()

	if _, retryAfter, err := limiter.RecordFailure(ctx, t.Name()); err != nil {
		t.Fatalf("record failure: %v", err)
	} else if retryAfter != int(testWindow.Seconds()) {
		t.Fatalf("first ttl = %d, want %d", retryAfter, int(testWindow.Seconds()))
	}

	ttl, err := client.TtlCtx(ctx, auth.PasswordFailureKeyPrefix+t.Name())
	if err != nil {
		t.Fatalf("ttl: %v", err)
	}
	if ttl <= 0 || ttl > int(testWindow.Seconds()) {
		t.Fatalf("ttl = %d, want (0, %d]", ttl, int(testWindow.Seconds()))
	}
	time.Sleep(1100 * time.Millisecond)

	if _, retryAfter, err := limiter.RecordFailure(ctx, t.Name()); err != nil {
		t.Fatalf("record second failure: %v", err)
	} else if retryAfter >= ttl {
		t.Fatalf("ttl after second failure = %d, want < %d (not reset)", retryAfter, ttl)
	}
}
