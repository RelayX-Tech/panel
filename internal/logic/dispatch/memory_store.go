package dispatch

import (
	"context"
	"errors"
	"sync"
)

// MemoryStore 是 Store 的内存实现（测试 / 本地演练）。
type MemoryStore struct {
	mu         sync.Mutex
	versions   map[string]uint64
	operations map[string]Operation
	snapshots  []Snapshot
}

// NewMemoryStore 构造内存 Store。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		versions:   make(map[string]uint64),
		operations: make(map[string]Operation),
	}
}

// AllocateVersion 返回节点下一个单调递增版本。
func (m *MemoryStore) AllocateVersion(_ context.Context, nodeID string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions[nodeID]++
	return m.versions[nodeID], nil
}

// SaveOperation 保存操作。
func (m *MemoryStore) SaveOperation(_ context.Context, op Operation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.operations[op.OperationID] = op
	return nil
}

// UpdateOperationState 更新操作状态。
func (m *MemoryStore) UpdateOperationState(_ context.Context, operationID, state, errorCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.operations[operationID]
	if !ok {
		return errors.New("operation not found")
	}
	op.State = state
	op.ErrorCode = errorCode
	m.operations[operationID] = op
	return nil
}

// SaveSnapshot 保存快照。
func (m *MemoryStore) SaveSnapshot(_ context.Context, snapshot Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = append(m.snapshots, snapshot)
	return nil
}

// Operation 返回操作（测试辅助）。
func (m *MemoryStore) Operation(operationID string) (Operation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.operations[operationID]
	return op, ok
}

// LatestSnapshot 返回某节点最近一次快照（测试辅助）。
func (m *MemoryStore) LatestSnapshot(nodeID string) (Snapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.snapshots) - 1; i >= 0; i-- {
		if m.snapshots[i].NodeID == nodeID {
			return m.snapshots[i], true
		}
	}
	return Snapshot{}, false
}
