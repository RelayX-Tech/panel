package dispatch

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AuditEntry 是一条配置下发审计记录。
type AuditEntry struct {
	Actor     string
	Action    string
	Object    string
	RequestID string
	Result    string
	CreatedAt time.Time
}

// AuditSink 记录配置下发审计（design §4.9 audit_logs）。
type AuditSink interface {
	Record(ctx context.Context, entry AuditEntry) error
}

// SQLAuditSink 是 AuditSink 的 PostgreSQL 实现。
type SQLAuditSink struct {
	db sqlx.SqlConn
}

// NewSQLAuditSink 构造审计写入器。
func NewSQLAuditSink(db sqlx.SqlConn) *SQLAuditSink {
	return &SQLAuditSink{db: db}
}

// Record 写入一条审计记录。
func (s *SQLAuditSink) Record(ctx context.Context, entry AuditEntry) error {
	_, err := s.db.ExecCtx(ctx,
		`INSERT INTO audit_logs (actor, action, object, request_id, result) VALUES ($1, $2, $3, $4, $5)`,
		entry.Actor, entry.Action, entry.Object, entry.RequestID, entry.Result)
	return err
}

var _ AuditSink = (*SQLAuditSink)(nil)
