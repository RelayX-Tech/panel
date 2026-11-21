package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 本文件是 auth_password_audits 的手写扩展查询（design.md §4.4）：
// 生成 model 只提供主键 CRUD；审计按 user_id / request_id 检索、按时间清理。
// 约束：client_ip 为 inet，读取统一用 host() 转文本，避免驱动扫描类型不匹配。

type (
	// PostgresAuthPasswordAuditModel 是 AuthPasswordAuditModel 的 PostgreSQL 实现。
	PostgresAuthPasswordAuditModel struct {
		conn      sqlx.SqlConn
		generated AuthPasswordAuditsModel
	}
)

var _ AuthPasswordAuditModel = (*PostgresAuthPasswordAuditModel)(nil)

// NewAuthPasswordAuditModel 构造审计适配层（不加 sqlcache：只追加、条件查询）。
func NewAuthPasswordAuditModel(conn sqlx.SqlConn) *PostgresAuthPasswordAuditModel {
	return &PostgresAuthPasswordAuditModel{
		conn:      conn,
		generated: NewAuthPasswordAuditsModel(conn),
	}
}

// Insert 写入一条修改成功审计；id / occurred_at 由数据库默认值生成。
func (m *PostgresAuthPasswordAuditModel) Insert(ctx context.Context, session sqlx.Session, audit *PasswordAudit) error {
	const query = `INSERT INTO auth_password_audits (user_id, request_id, client_ip, result)
VALUES ($1::uuid, $2, nullif($3, '')::inet, $4)`
	if _, err := execAffected(ctx, m.conn, session, query,
		audit.UserID, audit.RequestID, audit.ClientIP, audit.Result,
	); err != nil {
		return fmt.Errorf("insert password audit: %w", err)
	}
	return nil
}

// ListByUserID 按用户 ID 倒序返回审计（limit <= 0 时取 100）。
func (m *PostgresAuthPasswordAuditModel) ListByUserID(ctx context.Context, userID string, limit int) ([]PasswordAudit, error) {
	if limit <= 0 {
		limit = 100
	}
	const query = `SELECT id::text AS id, user_id::text AS user_id, request_id,
       coalesce(host(client_ip), '') AS client_ip, result, occurred_at
FROM auth_password_audits
WHERE user_id = $1::uuid
ORDER BY occurred_at DESC
LIMIT $2`
	return m.query(ctx, query, userID, limit)
}

// FindByRequestID 按 requestId 检索审计。
func (m *PostgresAuthPasswordAuditModel) FindByRequestID(ctx context.Context, requestID string) ([]PasswordAudit, error) {
	const query = `SELECT id::text AS id, user_id::text AS user_id, request_id,
       coalesce(host(client_ip), '') AS client_ip, result, occurred_at
FROM auth_password_audits
WHERE request_id = $1
ORDER BY occurred_at DESC`
	return m.query(ctx, query, requestID)
}

// DeleteBefore 删除 occurred_at 早于 before 的审计；可在事务内调用。
func (m *PostgresAuthPasswordAuditModel) DeleteBefore(ctx context.Context, session sqlx.Session, before time.Time) (int64, error) {
	const query = `DELETE FROM auth_password_audits WHERE occurred_at < $1`
	n, err := execAffected(ctx, m.conn, session, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete password audits: %w", err)
	}
	return n, nil
}

func (m *PostgresAuthPasswordAuditModel) query(ctx context.Context, query string, args ...any) ([]PasswordAudit, error) {
	var rows []AuthPasswordAudits
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("query password audits: %w", err)
	}
	audits := make([]PasswordAudit, 0, len(rows))
	for i := range rows {
		audits = append(audits, toDomainPasswordAudit(&rows[i]))
	}
	return audits, nil
}

func toDomainPasswordAudit(a *AuthPasswordAudits) PasswordAudit {
	return PasswordAudit{
		ID:         a.Id,
		UserID:     a.UserId,
		RequestID:  a.RequestId,
		ClientIP:   a.ClientIp.String,
		Result:     a.Result,
		OccurredAt: a.OccurredAt,
	}
}
