package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/relayx-tech/panel/internal/ws"
)

func TestHelloAndPublish(t *testing.T) {
	hub := ws.NewHub(nil)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.Serve(conn, strings.Split(r.URL.Query().Get("resources"), ","))
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "?resources=nodes,alerts"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var hello ws.Frame
	if err := conn.ReadJSON(&hello); err != nil {
		t.Fatalf("read hello: %v", err)
	}
	if hello.Type != "hello" || hello.SchemaVersion != 1 || hello.Seq == 0 {
		t.Fatalf("unexpected hello: %+v", hello)
	}

	frame := hub.Publish(context.Background(), "nodes", "status", map[string]any{"nodeId": "n1"})
	if frame.Seq <= hello.Seq {
		t.Fatalf("seq must increase: hello=%d frame=%d", hello.Seq, frame.Seq)
	}
	var got ws.Frame
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if got.Type != "status" || got.Seq != frame.Seq {
		t.Fatalf("unexpected frame: %+v", got)
	}
}

func TestPublishSkipsUnsubscribed(t *testing.T) {
	hub := ws.NewHub(nil)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.Serve(conn, []string{"alerts"})
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var hello ws.Frame
	_ = conn.ReadJSON(&hello)

	// 连接只订阅 alerts；nodes 帧不应到达。随后发 alerts 帧用于确认连接仍可用。
	hub.Publish(context.Background(), "nodes", "status", map[string]any{"nodeId": "n1"})
	hub.Publish(context.Background(), "alerts", "notification", map[string]any{"id": 1})
	var got ws.Frame
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Type != "notification" {
		t.Fatalf("expected notification, got %+v", got)
	}
}
