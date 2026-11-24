package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresClientModel struct {
	conn sqlx.SqlConn
}

func NewClientModel(conn sqlx.SqlConn) *PostgresClientModel {
	return &PostgresClientModel{conn: conn}
}

func (m *PostgresClientModel) Upsert(ctx context.Context, c ClientRow) error {
	const query = `INSERT INTO client_stats
    (client_id, port_id, port_group_id, email, up_bytes, down_bytes, last_online_at, last_sub_fetch_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (client_id) DO UPDATE SET
    port_id = excluded.port_id,
    port_group_id = excluded.port_group_id,
    email = excluded.email,
    up_bytes = excluded.up_bytes,
    down_bytes = excluded.down_bytes,
    last_online_at = excluded.last_online_at,
    last_sub_fetch_at = excluded.last_sub_fetch_at,
    updated_at = now()`
	if _, err := m.conn.ExecCtx(ctx, query, c.ClientID, c.PortID, c.PortGroupID, c.Email, c.UpBytes, c.DownBytes, timeArg(c.LastOnlineAt), timeArg(c.LastSubFetchAt)); err != nil {
		return fmt.Errorf("upsert client stats: %w", err)
	}
	return nil
}

func (m *PostgresClientModel) List(ctx context.Context, f ClientFilter) ([]ClientRow, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	var where []string
	var args []any
	if f.PortGroupID != "" {
		args = append(args, f.PortGroupID)
		where = append(where, fmt.Sprintf("port_group_id = $%d", len(args)))
	}
	if f.PortID != "" {
		args = append(args, f.PortID)
		where = append(where, fmt.Sprintf("port_id = $%d", len(args)))
	}
	if f.Query != "" {
		args = append(args, "%"+f.Query+"%")
		where = append(where, fmt.Sprintf("(email ILIKE $%d OR client_id ILIKE $%d)", len(args), len(args)))
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM client_stats"+whereSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count clients: %w", err)
	}
	query := `SELECT client_id, email, port_id, port_group_id, up_bytes, down_bytes, last_online_at, last_sub_fetch_at
FROM client_stats` + whereSQL + fmt.Sprintf(" ORDER BY (up_bytes + down_bytes) DESC, client_id ASC LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	type row struct {
		ClientId       string     `db:"client_id"`
		Email          string     `db:"email"`
		PortId         string     `db:"port_id"`
		PortGroupId    string     `db:"port_group_id"`
		UpBytes        int64      `db:"up_bytes"`
		DownBytes      int64      `db:"down_bytes"`
		LastOnlineAt   *time.Time `db:"last_online_at"`
		LastSubFetchAt *time.Time `db:"last_sub_fetch_at"`
	}
	var rows []row
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list clients: %w", err)
	}
	out := make([]ClientRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ClientRow{
			ClientID: r.ClientId, Email: r.Email, PortID: r.PortId, PortGroupID: r.PortGroupId,
			UpBytes: r.UpBytes, DownBytes: r.DownBytes,
			LastOnlineAt: r.LastOnlineAt, LastSubFetchAt: r.LastSubFetchAt,
		})
	}
	return out, total, nil
}

// OnlineSince 返回最近窗口内有流量的客户端数（控制台 clientsOnline）。
func (m *PostgresClientModel) OnlineSince(ctx context.Context, since time.Time) (int64, error) {
	var n int64
	const query = `SELECT count(*) FROM client_stats WHERE last_online_at >= $1`
	if err := m.conn.QueryRowCtx(ctx, &n, query, since); err != nil {
		return 0, fmt.Errorf("count online clients: %w", err)
	}
	return n, nil
}
