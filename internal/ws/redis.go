package ws

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisFanout 通过 Redis pub/sub 在多 Panel 实例间扇出。
type RedisFanout struct {
	client *goredis.Client
}

var _ Fanout = (*RedisFanout)(nil)

// NewRedisFanout 按地址与密码构造；addr 为空时返回 nil（单实例直连）。
func NewRedisFanout(addr, password string) *RedisFanout {
	if addr == "" {
		return nil
	}
	return &RedisFanout{client: goredis.NewClient(&goredis.Options{Addr: addr, Password: password})}
}

func (f *RedisFanout) Publish(ctx context.Context, channel string, data []byte) error {
	return f.client.Publish(ctx, channel, data).Err()
}

func (f *RedisFanout) Subscribe(ctx context.Context, pattern string) (<-chan Message, error) {
	pubsub := f.client.PSubscribe(ctx, pattern)
	out := make(chan Message, 256)
	go func() {
		defer close(out)
		defer pubsub.Close()
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				return
			}
			select {
			case out <- Message{Channel: msg.Channel, Data: []byte(msg.Payload)}:
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()
	return out, nil
}
