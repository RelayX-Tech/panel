package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// PasswordFailureKeyPrefix 是修改密码「旧密码连续错误」计数的 Redis key 前缀
// （design.md §4.2）。登录限流使用独立的进程内来源 IP 计数，两者互不影响。
const PasswordFailureKeyPrefix = "relayx:auth:pwd:fail:"

// passwordRecordScript 原子完成 INCR + 首次 EXPIRE + PTTL，返回 {count, ttlMillis}。
const passwordRecordScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {count, ttl}
`

// passwordCheckScript 返回 {count, ttlMillis}；key 不存在时 {0, 0}。
const passwordCheckScript = `
local count = redis.call('GET', KEYS[1])
if not count then
  return {0, 0}
end
local ttl = redis.call('PTTL', KEYS[1])
return {tonumber(count), ttl}
`

// PasswordLimiterStore 是 logic 依赖的限流能力（便于单测注入假实现）。
type PasswordLimiterStore interface {
	Check(ctx context.Context, userID string) (locked bool, retryAfterSeconds int, err error)
	RecordFailure(ctx context.Context, userID string) (count int, retryAfterSeconds int, err error)
	Reset(ctx context.Context, userID string) error
}

var _ PasswordLimiterStore = (*PasswordLimiter)(nil)

// PasswordLimiter 是修改密码旧密码错误的固定窗口限流器（Redis 共享计数）。
// 与登录限流（来源 IP、进程内）独立，成功修改后清零。
type PasswordLimiter struct {
	client      *redis.Redis
	window      time.Duration
	maxAttempts int
}

// NewPasswordLimiter 构造限流器；window/maxAttempts 必须为正（由 Config.Validate 保证）。
func NewPasswordLimiter(client *redis.Redis, window time.Duration, maxAttempts int) *PasswordLimiter {
	return &PasswordLimiter{client: client, window: window, maxAttempts: maxAttempts}
}

// Check 返回用户是否处于锁定窗口，以及剩余可重试秒数（至少 1）。
func (l *PasswordLimiter) Check(ctx context.Context, userID string) (bool, int, error) {
	result, err := l.client.EvalCtx(ctx, passwordCheckScript, []string{PasswordFailureKeyPrefix + userID})
	if err != nil {
		return false, 0, fmt.Errorf("check password limiter: %w", err)
	}
	count, ttlMillis, err := parseCounter(result)
	if err != nil {
		return false, 0, err
	}
	if count < l.maxAttempts {
		return false, 0, nil
	}
	return true, l.retryAfterSeconds(ttlMillis), nil
}

// RecordFailure 记录一次旧密码错误，返回累计次数与剩余秒数。
func (l *PasswordLimiter) RecordFailure(ctx context.Context, userID string) (int, int, error) {
	result, err := l.client.EvalCtx(ctx, passwordRecordScript,
		[]string{PasswordFailureKeyPrefix + userID}, l.window.Milliseconds())
	if err != nil {
		return 0, 0, fmt.Errorf("record password failure: %w", err)
	}
	count, ttlMillis, err := parseCounter(result)
	if err != nil {
		return 0, 0, err
	}
	return count, l.retryAfterSeconds(ttlMillis), nil
}

// Reset 清除计数；成功修改密码后调用。
func (l *PasswordLimiter) Reset(ctx context.Context, userID string) error {
	if _, err := l.client.DelCtx(ctx, PasswordFailureKeyPrefix+userID); err != nil {
		return fmt.Errorf("reset password limiter: %w", err)
	}
	return nil
}

// retryAfterSeconds 把 PTTL 毫秒换算为向上取整的秒数；异常 TTL（key 无过期）
// 回退到窗口长度，且恒不小于 1。
func (l *PasswordLimiter) retryAfterSeconds(ttlMillis int) int {
	if ttlMillis <= 0 {
		seconds := int(l.window.Seconds())
		if seconds < 1 {
			return 1
		}
		return seconds
	}
	seconds := (ttlMillis + 999) / 1000
	if seconds < 1 {
		return 1
	}
	return seconds
}

func parseCounter(result any) (int, int, error) {
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return 0, 0, fmt.Errorf("unexpected password limiter script result: %T", result)
	}
	count, ok := values[0].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected password limiter count: %T", values[0])
	}
	ttlMillis, ok := values[1].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected password limiter ttl: %T", values[1])
	}
	return int(count), int(ttlMillis), nil
}
