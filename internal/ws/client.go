package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client 是一条 WS 连接。
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan Frame
	resources map[string]bool

	closeOnce sync.Once
}

// enqueue 非阻塞写入；status/traffic 拥塞时丢弃最旧并追加 invalidate。
func (c *Client) enqueue(resource string, frame Frame) bool {
	select {
	case c.send <- frame:
		return true
	default:
	}
	switch frame.Type {
	case "status", "traffic":
		select {
		case <-c.send:
		default:
		}
		select {
		case c.send <- frame:
		default:
		}
		return true
	case "notification":
		// notification 不允许丢弃：交给写泵阻塞。
		c.send <- frame
		return true
	default:
		return false
	}
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		c.hub.remove(c)
		close(c.send)
		_ = c.conn.Close()
	})
}

func (c *Client) readPump() {
	defer c.close()
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.close()
	}()
	for {
		select {
		case frame, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(frame); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
