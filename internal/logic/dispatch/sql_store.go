package dispatch

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// SQLStore 是 Store 的 PostgreSQL 实现（design §4.9）。
type SQLStore struct {
	db sqlx.SqlConn
}

// NewSQLStore 构造 PostgreSQL Store。
func NewSQLStore(db sqlx.SqlConn) *SQLStore {
	return &SQLStore{db: db}
}

// AllocateVersion 在同一节点上以 advisory lock 串行分配单调版本。
func (s *SQLStore) AllocateVersion(ctx context.Context, nodeID string) (uint64, error) {
	var version int64
	err := s.db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, nodeID); err != nil {
			return err
		}
		return session.QueryRowCtx(ctx, &version,
			`SELECT COALESCE(MAX(version), 0) + 1 FROM config_operations WHERE node_id = $1`, nodeID)
	})
	if err != nil {
		return 0, err
	}
	return uint64(version), nil
}

// SaveOperation 写入 / 更新操作记录（幂等 upsert）。
func (s *SQLStore) SaveOperation(ctx context.Context, op Operation) error {
	_, err := s.db.ExecCtx(ctx, `
INSERT INTO config_operations (operation_id, node_id, version, type, state, request_id, error_code)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
ON CONFLICT (operation_id) DO UPDATE SET
  node_id = EXCLUDED.node_id,
  version = EXCLUDED.version,
  type = EXCLUDED.type,
  state = EXCLUDED.state,
  request_id = EXCLUDED.request_id,
  error_code = EXCLUDED.error_code,
  updated_at = now()`,
		op.OperationID, op.NodeID, op.Version, op.Type, op.State, op.RequestID, op.ErrorCode)
	return err
}

// UpdateOperationState 更新操作状态。
func (s *SQLStore) UpdateOperationState(ctx context.Context, operationID, state, errorCode string) error {
	_, err := s.db.ExecCtx(ctx,
		`UPDATE config_operations SET state = $2, error_code = $3, updated_at = now() WHERE operation_id = $1::uuid`,
		operationID, state, errorCode)
	return err
}

// SaveSnapshot 保存加密回滚快照。
func (s *SQLStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	_, err := s.db.ExecCtx(ctx,
		`INSERT INTO config_snapshots (node_id, version, payload_encrypted, hash, reason) VALUES ($1, $2, $3, $4, $5)`,
		snapshot.NodeID, snapshot.Version, snapshot.Payload, snapshot.Hash, snapshot.Reason)
	return err
}

var _ Store = (*SQLStore)(nil)
