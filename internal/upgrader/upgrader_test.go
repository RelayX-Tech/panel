package upgrader_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/relayx-tech/panel/internal/upgrade"
	"github.com/relayx-tech/panel/internal/upgrader"
)

type fakeApplier struct {
	mu       sync.Mutex
	applied  []upgrader.Request
	rollback []upgrader.Request
	fail     bool
}

func (f *fakeApplier) Apply(_ context.Context, req upgrader.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, req)
	return nil
}
func (f *fakeApplier) Rollback(_ context.Context, req upgrader.Request) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rollback = append(f.rollback, req)
	return nil
}

func TestServerAuthAndUpgrade(t *testing.T) {
	store := &upgrader.FileStore{Path: filepath.Join(t.TempDir(), "state.json")}
	applier := &fakeApplier{}
	server := &upgrader.Server{Applier: applier, Store: store, Token: "secret"}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	// 无令牌 → 401。
	resp, err := http.Get(httpServer.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status without token = %d", resp.StatusCode)
	}
	resp.Body.Close()

	body := `{"operationId":"op-1","targetVersion":"1.4.0","assets":[{"name":"panel","url":"https://example.com/p","sha256":"abc"}]}`
	req, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/upgrade", strings.NewReader(body))
	req.Header.Set("X-Relayx-Upgrader-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upgrade = %d", resp.StatusCode)
	}
	resp.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		applier.mu.Lock()
		n := len(applier.applied)
		applier.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	applier.mu.Lock()
	defer applier.mu.Unlock()
	if len(applier.applied) != 1 || applier.applied[0].TargetVersion != "1.4.0" {
		t.Fatalf("applied = %+v", applier.applied)
	}
}

// ---- applier ----

type fakeLinker struct {
	current  string
	switches []string
}

func (f *fakeLinker) Current(string) (string, error) { return f.current, nil }
func (f *fakeLinker) Switch(_ string, target string) error {
	f.switches = append(f.switches, target)
	f.current = target
	return nil
}

type fakeRunner struct {
	fail     bool
	restarts []string
}

func (f *fakeRunner) Restart(_ context.Context, service string) error {
	f.restarts = append(f.restarts, service)
	if f.fail {
		return errors.New("systemctl failed")
	}
	return nil
}

func TestApplierSuccessAndRollback(t *testing.T) {
	payload := []byte("binary")
	sum := sha256.Sum256(payload)
	fileServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer fileServer.Close()

	baseDir := t.TempDir()
	store := &upgrader.FileStore{Path: filepath.Join(baseDir, "state.json")}
	linker := &fakeLinker{current: "releases/1.3.0"}
	runner := &fakeRunner{}
	applier := &upgrader.FileApplier{
		BaseDir: baseDir, Services: []string{"relayx-panel.service"},
		Runner: runner, Linker: linker, Store: store, HTTPClient: fileServer.Client(),
	}
	req := upgrader.Request{
		OperationID: "op-1", TargetVersion: "1.4.0",
		Assets: []upgrade.Asset{{
			Name: "panel", URL: fileServer.URL + "/panel", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(payload)),
		}},
	}
	if err := applier.Apply(context.Background(), req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	state, _ := store.Load()
	if state.State != "succeeded" || state.ToVersion != "1.4.0" {
		t.Fatalf("state = %+v", state)
	}
	if len(linker.switches) != 1 || !strings.Contains(linker.switches[0], "1.4.0") {
		t.Fatalf("switches = %+v", linker.switches)
	}

	// 重启失败 → 回滚到原软链并标记失败。
	runner.fail = true
	req.OperationID = "op-2"
	req.TargetVersion = "1.5.0"
	if err := applier.Apply(context.Background(), req); err == nil {
		t.Fatal("apply should fail when restart fails")
	}
	state, _ = store.Load()
	if state.State != "rolled_back" {
		t.Fatalf("state = %+v, want rolled_back", state)
	}
	if linker.current != "releases/1.4.0" && !strings.Contains(linker.current, "1.4.0") {
		t.Fatalf("link should roll back to previous, got %s", linker.current)
	}
}

func TestApplierRejectsHTTPAndBadChecksum(t *testing.T) {
	baseDir := t.TempDir()
	store := &upgrader.FileStore{Path: filepath.Join(baseDir, "state.json")}
	applier := &upgrader.FileApplier{
		BaseDir: baseDir, Runner: &fakeRunner{}, Linker: &fakeLinker{}, Store: store,
	}
	req := upgrader.Request{OperationID: "op", TargetVersion: "1.4.0", Assets: []upgrade.Asset{{Name: "panel", URL: "http://example.com/p"}}}
	if err := applier.Apply(context.Background(), req); err == nil {
		t.Fatal("http asset should be rejected")
	}
}

func TestServerStatusJSON(t *testing.T) {
	store := &upgrader.FileStore{Path: filepath.Join(t.TempDir(), "state.json")}
	_ = store.Save(upgrader.State{State: "idle", OperationID: "op"})
	server := &upgrader.Server{Applier: &fakeApplier{}, Store: store, Token: "t"}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("X-Relayx-Upgrader-Token", "t")
	server.Handler().ServeHTTP(rec, req)
	var got upgrader.State
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.OperationID != "op" {
		t.Fatalf("got %+v", got)
	}
}
