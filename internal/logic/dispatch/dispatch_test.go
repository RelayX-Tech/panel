package dispatch_test

import (
	"context"
	"testing"

	"github.com/relayx-tech/panel/internal/cryptobox"
	"github.com/relayx-tech/panel/internal/logic/dispatch"
)

func newService(t *testing.T) (*dispatch.Service, *dispatch.MemoryStore) {
	t.Helper()
	box, err := cryptobox.New(make([]byte, 32), "k1")
	if err != nil {
		t.Fatal(err)
	}
	store := dispatch.NewMemoryStore()
	return dispatch.New(store, box), store
}

func TestBeginOperationAllocatesMonotonicVersion(t *testing.T) {
	svc, store := newService(t)
	op1, err := svc.BeginOperation(context.Background(), dispatch.Operation{OperationID: "op-1", NodeID: "n1", Type: "apply"})
	if err != nil {
		t.Fatal(err)
	}
	op2, err := svc.BeginOperation(context.Background(), dispatch.Operation{OperationID: "op-2", NodeID: "n1", Type: "apply"})
	if err != nil {
		t.Fatal(err)
	}
	if op1.Version != 1 || op2.Version != 2 {
		t.Fatalf("versions = %d, %d", op1.Version, op2.Version)
	}
	if op1.State != dispatch.StatePending {
		t.Fatalf("state = %q", op1.State)
	}
	if got, ok := store.Operation("op-1"); !ok || got.Version != 1 {
		t.Fatalf("stored = %+v", got)
	}
}

func TestTransition(t *testing.T) {
	svc, store := newService(t)
	op, err := svc.BeginOperation(context.Background(), dispatch.Operation{OperationID: "op-1", NodeID: "n1", Type: "apply"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Transition(context.Background(), op.OperationID, dispatch.StateApplied, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Operation("op-1"); got.State != dispatch.StateApplied {
		t.Fatalf("state = %q", got.State)
	}
}

func TestSnapshotEncryptedRoundTrip(t *testing.T) {
	svc, store := newService(t)
	plaintext := []byte(`{"inbounds":[]}`)
	if err := svc.RecordSnapshot(context.Background(), "n1", 1, plaintext, "hash", "deploy"); err != nil {
		t.Fatal(err)
	}
	snap, ok := store.LatestSnapshot("n1")
	if !ok {
		t.Fatal("snapshot 未保存")
	}
	if string(snap.Payload) == string(plaintext) {
		t.Fatal("快照应为密文")
	}
	got, err := svc.DecryptSnapshot(snap.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("解密 = %q", got)
	}
}
