package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresTrafficModel struct {
	conn sqlx.SqlConn
}

func NewTrafficModel(conn sqlx.SqlConn) *PostgresTrafficModel {
	return &PostgresTrafficModel{conn: conn}
}

func (m *PostgresTrafficModel) UpsertSample(ctx context.Context, s TrafficSample) error {
	const query = `INSERT INTO traffic_samples (entity_type, entity_id, ts, up_bytes, down_bytes, connections)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (entity_type, entity_id, ts) DO UPDATE
SET up_bytes = excluded.up_bytes, down_bytes = excluded.down_bytes, connections = excluded.connections`
	if _, err := m.conn.ExecCtx(ctx, query, s.EntityType, s.EntityID, s.TS, s.UpBytes, s.DownBytes, s.Connections); err != nil {
		return fmt.Errorf("upsert traffic sample: %w", err)
	}
	return nil
}

// Series 按步长选择原始 / 分钟 / 小时表。
func (m *PostgresTrafficModel) Series(ctx context.Context, entityType, entityID, metric string, from, to time.Time, step time.Duration) ([]TrafficPoint, error) {
	table := "traffic_samples"
	switch {
	case step >= 24*time.Hour:
		table = "traffic_1h"
	case step >= time.Hour:
		table = "traffic_1m"
	}
	query := fmt.Sprintf(`SELECT ts, up_bytes, down_bytes, connections FROM %s
WHERE entity_type = $1 AND entity_id = $2 AND ts >= $3 AND ts <= $4 ORDER BY ts ASC`, table)
	type row struct {
		Ts          time.Time `db:"ts"`
		UpBytes     int64     `db:"up_bytes"`
		DownBytes   int64     `db:"down_bytes"`
		Connections int       `db:"connections"`
	}
	var rows []row
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, entityType, entityID, from, to); err != nil {
		return nil, fmt.Errorf("traffic series: %w", err)
	}
	out := make([]TrafficPoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, TrafficPoint{TS: r.Ts, UpBytes: r.UpBytes, DownBytes: r.DownBytes, Connections: r.Connections})
	}
	return out, nil
}

func (m *PostgresTrafficModel) Rows(ctx context.Context, entityType string, from, to time.Time, page, pageSize int, sort string) ([]TrafficRow, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	where := []string{"ts >= $1", "ts <= $2"}
	args := []any{from, to}
	if entityType != "" {
		args = append(args, entityType)
		where = append(where, fmt.Sprintf("entity_type = $%d", len(args)))
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total,
		"SELECT count(*) FROM (SELECT entity_type, entity_id FROM traffic_samples"+whereSQL+" GROUP BY entity_type, entity_id) t", args...); err != nil {
		return nil, 0, fmt.Errorf("count traffic rows: %w", err)
	}

	order := " ORDER BY total_bytes DESC, entity_id ASC"
	switch sort {
	case "up":
		order = " ORDER BY up_bytes DESC, entity_id ASC"
	case "down":
		order = " ORDER BY down_bytes DESC, entity_id ASC"
	case "connections":
		order = " ORDER BY connections DESC, entity_id ASC"
	}
	// C19：objectName 回填实体显示名（端口组 / 节点 / 客户端），无法解析时回退 entity_id。
	query := `SELECT g.entity_type, g.entity_id,
       coalesce(pg.name, n.name, nullif(cs.email, ''), g.entity_id) AS object_name,
       g.up_bytes, g.down_bytes, g.total_bytes, g.connections, g.last_updated_at
FROM (
    SELECT entity_type, entity_id,
           sum(up_bytes)::bigint AS up_bytes,
           sum(down_bytes)::bigint AS down_bytes,
           sum(up_bytes + down_bytes)::bigint AS total_bytes,
           max(connections)::int AS connections,
           max(ts) AS last_updated_at
    FROM traffic_samples` + whereSQL + `
    GROUP BY entity_type, entity_id
) g
LEFT JOIN port_groups pg ON g.entity_type = 'port_group' AND pg.id = g.entity_id
LEFT JOIN nodes n ON g.entity_type = 'node' AND n.id = g.entity_id
LEFT JOIN client_stats cs ON g.entity_type = 'client' AND cs.client_id = g.entity_id` + order + fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	type row struct {
		EntityType    string    `db:"entity_type"`
		EntityId      string    `db:"entity_id"`
		ObjectName    string    `db:"object_name"`
		UpBytes       int64     `db:"up_bytes"`
		DownBytes     int64     `db:"down_bytes"`
		TotalBytes    int64     `db:"total_bytes"`
		Connections   int       `db:"connections"`
		LastUpdatedAt time.Time `db:"last_updated_at"`
	}
	var rows []row
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("traffic rows: %w", err)
	}
	out := make([]TrafficRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, TrafficRow{
			ObjectType: r.EntityType, ObjectID: r.EntityId, ObjectName: r.ObjectName,
			UpBytes: r.UpBytes, DownBytes: r.DownBytes, TotalBytes: r.TotalBytes,
			Connections: r.Connections, LastUpdatedAt: r.LastUpdatedAt,
		})
	}
	return out, total, nil
}

func (m *PostgresTrafficModel) Totals(ctx context.Context, from, to time.Time) (int64, int64, int, error) {
	const query = `SELECT coalesce(sum(up_bytes),0)::bigint AS up,
       coalesce(sum(down_bytes),0)::bigint AS down,
       coalesce(max(connections),0)::int AS conns
FROM traffic_samples WHERE ts >= $1 AND ts <= $2`
	var counts struct {
		Up    int64 `db:"up"`
		Down  int64 `db:"down"`
		Conns int   `db:"conns"`
	}
	if err := m.conn.QueryRowCtx(ctx, &counts, query, from, to); err != nil {
		return 0, 0, 0, fmt.Errorf("traffic totals: %w", err)
	}
	return counts.Up, counts.Down, counts.Conns, nil
}

func (m *PostgresTrafficModel) StorageStats(ctx context.Context) (StorageStats, error) {
	const query = `SELECT
  pg_database_size(current_database())::bigint AS db_size,
  coalesce(sum(pg_total_relation_size(c.oid)), 0)::bigint AS traffic_size
FROM pg_class c
WHERE c.relname LIKE 'traffic_%' AND c.relkind IN ('r', 'p')`
	var row struct {
		DbSize      int64 `db:"db_size"`
		TrafficSize int64 `db:"traffic_size"`
	}
	if err := m.conn.QueryRowCtx(ctx, &row, query); err != nil {
		return StorageStats{}, fmt.Errorf("storage stats: %w", err)
	}
	stats := StorageStats{DatabaseSizeBytes: row.DbSize, TrafficPartitionSizeBytes: row.TrafficSize}
	const bounds = `SELECT min(ts) AS earliest, max(ts) AS latest FROM traffic_samples`
	var b struct {
		Earliest *time.Time `db:"earliest"`
		Latest   *time.Time `db:"latest"`
	}
	if err := m.conn.QueryRowCtx(ctx, &b, bounds); err == nil {
		stats.EarliestRecordAt = b.Earliest
		stats.LatestRecordAt = b.Latest
	}
	return stats, nil
}
