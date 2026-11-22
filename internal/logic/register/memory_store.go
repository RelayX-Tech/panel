package register

import (
	"context"
	"sync"
	"time"
)

// MemoryStore 是 Store 的内存实现（测试 / 本地）。
type MemoryStore struct {
	mu     sync.Mutex
	tokens map[string]TokenRecord
	certs  map[string]CertRecord
}

// NewMemoryStore 构造内存 Store。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{tokens: make(map[string]TokenRecord), certs: make(map[string]CertRecord)}
}

func (m *MemoryStore) CreateToken(_ context.Context, rec TokenRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[rec.Hash] = rec
	return nil
}

func (m *MemoryStore) GetToken(_ context.Context, hash string) (TokenRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.tokens[hash]
	return rec, ok, nil
}

func (m *MemoryStore) MarkTokenUsed(_ context.Context, hash string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.tokens[hash]
	rec.UsedAt = at
	m.tokens[hash] = rec
	return nil
}

func (m *MemoryStore) GetCert(_ context.Context, nodeID string) (CertRecord, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.certs[nodeID]
	return rec, ok, nil
}

func (m *MemoryStore) SaveCert(_ context.Context, rec CertRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.certs[rec.NodeID] = rec
	return nil
}

func (m *MemoryStore) RevokeCert(_ context.Context, nodeID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.certs[nodeID]
	rec.RevokedAt = at
	m.certs[nodeID] = rec
	return nil
}
