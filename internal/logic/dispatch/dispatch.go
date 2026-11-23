// Package dispatch 管理下发操作状态机、版本分配与加密快照（design §4.7 / §4.9）。
package dispatch

import (
	"context"
	"time"

	"github.com/relayx-tech/panel/internal/cryptobox"
)

// 操作状态。
const (
	StatePending    = "PENDING"
	StateGenerating = "GENERATING"
	StateApplying   = "APPLYING"
	StateApplied    = "APPLIED"
	StateFailed     = "FAILED"
	StateRolledBack = "ROLLED_BACK"
)

// Operation 是一条下发操作。
type Operation struct {
	OperationID string
	NodeID      string
	Version     uint64
	Type        string
	State       string
	RequestID   string
	ErrorCode   string
	CreatedAt   time.Time
}

// Snapshot 是回滚快照（Payload 为加密后的字节）。
type Snapshot struct {
	NodeID    string
	Version   uint64
	Payload   []byte
	Hash      string
	Reason    string
	CreatedAt time.Time
}

// Store 是下发持久化抽象（真实实现走 PostgreSQL；测试用内存实现）。
type Store interface {
	AllocateVersion(ctx context.Context, nodeID string) (uint64, error)
	SaveOperation(ctx context.Context, op Operation) error
	UpdateOperationState(ctx context.Context, operationID, state, errorCode string) error
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error
}

// Service 编排操作状态、版本与快照。
type Service struct {
	store Store
	box   *cryptobox.Box
	now   func() time.Time
}

// New 构造 Service。
func New(store Store, box *cryptobox.Box) *Service {
	return &Service{store: store, box: box, now: time.Now}
}

// BeginOperation 分配单调版本并写入 PENDING 操作（短事务，不持锁跨 RPC）。
func (s *Service) BeginOperation(ctx context.Context, op Operation) (Operation, error) {
	if op.Version == 0 {
		version, err := s.store.AllocateVersion(ctx, op.NodeID)
		if err != nil {
			return Operation{}, err
		}
		op.Version = version
	}
	op.State = StatePending
	op.CreatedAt = s.now()
	if err := s.store.SaveOperation(ctx, op); err != nil {
		return Operation{}, err
	}
	return op, nil
}

// Transition 更新操作状态。
func (s *Service) Transition(ctx context.Context, operationID, state, errorCode string) error {
	return s.store.UpdateOperationState(ctx, operationID, state, errorCode)
}

// RecordSnapshot 加密并保存回滚快照（AES-256-GCM）。
func (s *Service) RecordSnapshot(ctx context.Context, nodeID string, version uint64, plaintext []byte, hash, reason string) error {
	sealed, err := s.box.Seal(plaintext)
	if err != nil {
		return err
	}
	return s.store.SaveSnapshot(ctx, Snapshot{
		NodeID: nodeID, Version: version, Payload: sealed, Hash: hash, Reason: reason, CreatedAt: s.now(),
	})
}

// DecryptSnapshot 解密快照。
func (s *Service) DecryptSnapshot(payload []byte) ([]byte, error) {
	return s.box.Open(payload)
}
