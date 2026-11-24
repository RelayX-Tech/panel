package upgrade_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/upgrade"
)

func TestUpgraderStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Relayx-Upgrader-Token") != "secret" {
			t.Errorf("missing token")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state": "idle", "operationId": "op-1", "fromVersion": "1.3.0", "toVersion": "1.4.0",
		})
	}))
	defer server.Close()

	client := &upgrade.UpgraderHTTP{Endpoint: server.URL, Token: "secret", HTTP: &http.Client{Timeout: time.Second}}
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.OperationID != "op-1" || status.ToVersion != "1.4.0" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestUpgraderUpgrade(t *testing.T) {
	var got upgrade.PanelUpgradeRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true, "state": "running"})
	}))
	defer server.Close()

	client := &upgrade.UpgraderHTTP{Endpoint: server.URL, Token: "secret", HTTP: &http.Client{Timeout: time.Second}}
	req := upgrade.PanelUpgradeRequest{OperationID: "op-1", TargetVersion: "1.4.0", Assets: []upgrade.Asset{{Name: "panel", SHA256: "abc"}}}
	if err := client.Upgrade(context.Background(), req); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if got.OperationID != "op-1" || len(got.Assets) != 1 {
		t.Fatalf("unexpected request: %+v", got)
	}
}
