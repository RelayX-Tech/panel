package notify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relayx-tech/panel/internal/notify"
)

func TestSendWebhook(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := notify.NewClient().Send(context.Background(), "webhook", server.URL, "hello"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotBody == "" {
		t.Fatal("webhook body empty")
	}
}

func TestSendWebhookFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	if err := notify.NewClient().Send(context.Background(), "webhook", server.URL, "hello"); err == nil {
		t.Fatal("5xx should fail")
	}
}

func TestSendTelegramInvalidTarget(t *testing.T) {
	if err := notify.NewClient().Send(context.Background(), "telegram", "invalid", "hi"); err == nil {
		t.Fatal("invalid target should fail")
	}
}
