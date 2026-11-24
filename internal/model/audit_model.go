package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresAuditLogModel struct {
	conn sqlx.SqlConn
}

func NewAuditLogModel(conn sqlx.SqlConn) *PostgresAuditLogModel {
	return &PostgresAuditLogModel{conn: conn}
}

func (m *PostgresAuditLogModel) Insert(ctx context.Context, l AuditLog) error {
	object := l.ObjectType
	if l.ObjectID != "" {
		object = l.ObjectType + ":" + l.ObjectID
	}
	const query = `INSERT INTO audit_logs (actor, action, object, object_type, object_id, request_id, result, ip, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := m.conn.ExecCtx(ctx, query, l.Actor, l.Action, object, l.ObjectType, l.ObjectID, l.RequestID, l.Result, l.IP, l.CreatedAt); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

const auditSelect = `SELECT id, actor, action, object_type, object_id, request_id, result, ip, created_at FROM audit_logs`

type auditRow struct {
	Id         int64     `db:"id"`
	Actor      string    `db:"actor"`
	Action     string    `db:"action"`
	ObjectType string    `db:"object_type"`
	ObjectId   string    `db:"object_id"`
	RequestId  string    `db:"request_id"`
	Result     string    `db:"result"`
	Ip         string    `db:"ip"`
	CreatedAt  time.Time `db:"created_at"`
}

func (r auditRow) log() AuditLog {
	return AuditLog{ID: r.Id, Actor: r.Actor, Action: r.Action, ObjectType: r.ObjectType, ObjectID: r.ObjectId,
		RequestID: r.RequestId, Result: r.Result, IP: r.Ip, CreatedAt: r.CreatedAt}
}

func (m *PostgresAuditLogModel) List(ctx context.Context, f AuditFilter) ([]AuditLog, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	var where []string
	var args []any
	add := func(column, value string) {
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if f.Actor != "" {
		add("actor", f.Actor)
	}
	if f.Action != "" {
		add("action", f.Action)
	}
	if f.ObjectType != "" {
		add("object_type", f.ObjectType)
	}
	if f.ObjectID != "" {
		add("object_id", f.ObjectID)
	}
	if f.RequestID != "" {
		add("request_id", f.RequestID)
	}
	if f.From != nil {
		args = append(args, *f.From)
		where = append(where, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where = append(where, fmt.Sprintf("created_at <= $%d", len(args)))
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM audit_logs"+whereSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}
	query := auditSelect + whereSQL + fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	var rows []auditRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	out := make([]AuditLog, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.log())
	}
	return out, total, nil
}

func (m *PostgresAuditLogModel) Get(ctx context.Context, id int64) (*AuditLog, error) {
	var row auditRow
	if err := m.conn.QueryRowCtx(ctx, &row, auditSelect+" WHERE id = $1", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get audit log: %w", err)
	}
	l := row.log()
	return &l, nil
}

func (m *PostgresAuditLogModel) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	const query = `DELETE FROM audit_logs WHERE created_at < $1`
	result, err := m.conn.ExecCtx(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("delete audit logs: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}
