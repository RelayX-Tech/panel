// Package ws 实现 Panel 的实时推送 Hub（design.md §5.2）。
package ws

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Frame 是 WS 帧信封（《观测与实时推送协议》）。
type Frame struct {
	Type          string `json:"type"`
	SchemaVersion int    `json:"schemaVersion,omitempty"`
	Seq           int64  `json:"seq"`
	TS            string `json:"ts"`
	Payload       any    `json:"payload,omitempty"`
}

// Message 是扇出消息。
type Message struct {
	Channel string
	Data    []byte
}

// Fanout 抽象多实例 Redis pub/sub。
type Fanout interface {
	Publish(ctx context.Context, channel string, data []byte) error
	Subscribe(ctx context.Context, pattern string) (<-chan Message, error)
}

const (
	sendBuffer   = 64
	pingInterval = 15 * time.Second
	pongWait     = 30 * time.Second
)

// Hub 管理连接、资源订阅与全局 seq。
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]struct{}
	seq        atomic.Int64
	instanceID string
	fanout     Fanout
}

func NewHub(fanout Fanout) *Hub {
	return &Hub{clients: make(map[*Client]struct{}), instanceID: uuid.NewString(), fanout: fanout}
}

// InstanceID 返回实例标识（用于扇出去重）。
func (h *Hub) InstanceID() string { return h.instanceID }

// Seq 返回当前序号。
func (h *Hub) Seq() int64 { return h.seq.Load() }

// Serve 接受一条已升级的连接，发送 hello 并开始收发泵（阻塞至连接关闭）。
func (h *Hub) Serve(conn *websocket.Conn, resources []string) {
	client := &Client{
		hub:       h,
		conn:      conn,
		send:      make(chan Frame, sendBuffer),
		resources: make(map[string]bool, len(resources)),
	}
	for _, r := range resources {
		client.resources[r] = true
	}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	h.seq.Add(1)
	hello := Frame{
		Type: "hello", SchemaVersion: 1, Seq: h.seq.Load(), TS: timestamp(),
		Payload: map[string]any{"resources": resources},
	}
	_ = conn.WriteJSON(hello)

	go client.writePump()
	client.readPump()
}

// Publish 广播一帧：本地投递 + 多实例扇出。
func (h *Hub) Publish(ctx context.Context, resource, frameType string, payload any) Frame {
	frame := Frame{Type: frameType, Seq: h.seq.Add(1), TS: timestamp(), Payload: payload}
	h.broadcastLocal(resource, frame)
	if h.fanout != nil {
		envelope, _ := json.Marshal(map[string]any{
			"instance": h.instanceID, "resource": resource, "frame": frame,
		})
		if err := h.fanout.Publish(ctx, fanoutChannel(resource), envelope); err != nil {
			// 扇出失败不影响本地投递。
			_ = err
		}
	}
	return frame
}

// Run 订阅 Redis 扇出频道；阻塞直到 ctx 结束。
func (h *Hub) Run(ctx context.Context) {
	if h.fanout == nil {
		return
	}
	messages, err := h.fanout.Subscribe(ctx, "relayx:ws:*")
	if err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}
			var envelope struct {
				Instance string          `json:"instance"`
				Resource string          `json:"resource"`
				Frame    json.RawMessage `json:"frame"`
			}
			if json.Unmarshal(msg.Data, &envelope) != nil || envelope.Instance == h.instanceID {
				continue
			}
			var frame Frame
			if json.Unmarshal(envelope.Frame, &frame) != nil {
				continue
			}
			h.broadcastLocal(envelope.Resource, frame)
		}
	}
}

func (h *Hub) broadcastLocal(resource string, frame Frame) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		if len(c.resources) == 0 || c.resources[resource] {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range clients {
		if !c.enqueue(resource, frame) {
			c.close()
			continue
		}
		// seq 跳号由客户端检测后触发 invalidate；服务端在拥塞时主动补发。
	}
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func fanoutChannel(resource string) string { return "relayx:ws:" + resource }

func timestamp() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00") }
