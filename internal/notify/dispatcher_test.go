package notify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/notify"
)

type fakeBox struct{ target string }

func (b fakeBox) Open([]byte) ([]byte, error) { return []byte(b.target), nil }

type fakeChannels struct {
	channels []model.NotificationChannel
	sent     []int64
}

func (f *fakeChannels) List(context.Context) ([]model.NotificationChannel, error) {
	return f.channels, nil
}
func (f *fakeChannels) MarkSent(_ context.Context, id int64, _ time.Time) error {
	f.sent = append(f.sent, id)
	return nil
}

type fakeRecorder struct{ records []model.AlertNotification }

func (f *fakeRecorder) InsertNotification(_ context.Context, n model.AlertNotification) error {
	f.records = append(f.records, n)
	return nil
}

func TestDispatcherLevelFiltering(t *testing.T) {
	delivered := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		delivered++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channels := &fakeChannels{channels: []model.NotificationChannel{
		{ID: 1, Type: "webhook", TargetEncrypted: []byte("x"), MinLevel: "warning", Enabled: true},
		{ID: 2, Type: "webhook", TargetEncrypted: []byte("x"), MinLevel: "critical", Enabled: true},
		{ID: 3, Type: "webhook", TargetEncrypted: []byte("x"), MinLevel: "info", Enabled: false},
	}}
	recorder := &fakeRecorder{}
	dispatcher := &notify.Dispatcher{
		Channels: channels, Alerts: recorder, Box: fakeBox{target: server.URL},
		Sender: notify.NewClient(),
	}
	dispatcher.Dispatch(context.Background(), 42, "warning", "node failed")

	if delivered != 1 {
		t.Fatalf("delivered = %d, want 1 (only channel 1 matches warning)", delivered)
	}
	if len(recorder.records) != 1 || recorder.records[0].ChannelID != 1 {
		t.Fatalf("records = %+v", recorder.records)
	}
	if recorder.records[0].Status != "sent" {
		t.Fatalf("status = %s", recorder.records[0].Status)
	}
	if len(channels.sent) != 1 || channels.sent[0] != 1 {
		t.Fatalf("MarkSent = %+v", channels.sent)
	}
}
