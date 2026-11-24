package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresAlertModel struct {
	conn sqlx.SqlConn
}

func NewAlertModel(conn sqlx.SqlConn) *PostgresAlertModel {
	return &PostgresAlertModel{conn: conn}
}

// Upsert 按（type, object_type, object_id, active）收敛告警：存在则刷新，否则新建。
func (m *PostgresAlertModel) Upsert(ctx context.Context, a Alert) (*Alert, error) {
	var id int64
	const update = `UPDATE alerts SET message = $1, level = $2, last_seen_at = $3, status = $4, resolved_at = NULL
WHERE type = $5 AND object_type = $6 AND object_id = $7 AND status = $4
RETURNING id`
	err := m.conn.QueryRowCtx(ctx, &id, update, a.Message, a.Level, a.LastSeenAt, AlertStatusActive, a.Type, a.ObjectType, a.ObjectID)
	if err == nil {
		return m.Get(ctx, id)
	}
	if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, fmt.Errorf("update alert: %w", err)
	}

	const insert = `INSERT INTO alerts (type, level, object_type, object_id, message, status, first_seen_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	if err := m.conn.QueryRowCtx(ctx, &id, insert, a.Type, a.Level, a.ObjectType, a.ObjectID, a.Message, a.Status, a.FirstSeenAt, a.LastSeenAt); err != nil {
		return nil, fmt.Errorf("insert alert: %w", err)
	}
	return m.Get(ctx, id)
}

const alertSelect = `SELECT id, type, level, object_type, object_id, message, status, first_seen_at, last_seen_at, resolved_at
FROM alerts`

type alertRow struct {
	Id          int64      `db:"id"`
	Type        string     `db:"type"`
	Level       string     `db:"level"`
	ObjectType  string     `db:"object_type"`
	ObjectId    string     `db:"object_id"`
	Message     string     `db:"message"`
	Status      string     `db:"status"`
	FirstSeenAt time.Time  `db:"first_seen_at"`
	LastSeenAt  time.Time  `db:"last_seen_at"`
	ResolvedAt  *time.Time `db:"resolved_at"`
}

func (r alertRow) alert() Alert {
	return Alert{
		ID: r.Id, Type: r.Type, Level: r.Level, ObjectType: r.ObjectType, ObjectID: r.ObjectId,
		ObjectName: r.ObjectId, Message: r.Message, Status: r.Status,
		FirstSeenAt: r.FirstSeenAt, LastSeenAt: r.LastSeenAt, ResolvedAt: r.ResolvedAt,
	}
}

func (m *PostgresAlertModel) List(ctx context.Context, f AlertFilter) ([]Alert, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	var where []string
	var args []any
	if f.Level != "" {
		args = append(args, f.Level)
		where = append(where, fmt.Sprintf("level = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Type != "" {
		args = append(args, f.Type)
		where = append(where, fmt.Sprintf("type = $%d", len(args)))
	}
	if f.ObjectType != "" {
		args = append(args, f.ObjectType)
		where = append(where, fmt.Sprintf("object_type = $%d", len(args)))
	}
	if f.Query != "" {
		args = append(args, "%"+f.Query+"%")
		where = append(where, fmt.Sprintf("(message ILIKE $%d OR object_id ILIKE $%d)", len(args), len(args)))
	}
	if f.From != nil {
		args = append(args, *f.From)
		where = append(where, fmt.Sprintf("last_seen_at >= $%d", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where = append(where, fmt.Sprintf("last_seen_at <= $%d", len(args)))
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM alerts"+whereSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count alerts: %w", err)
	}

	order := " ORDER BY last_seen_at DESC, id DESC"
	switch f.Sort {
	case "level":
		order = " ORDER BY level ASC, last_seen_at DESC"
	case "lastSeen":
		order = " ORDER BY last_seen_at DESC, id DESC"
	}
	query := alertSelect + whereSQL + order + fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	var rows []alertRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list alerts: %w", err)
	}
	out := make([]Alert, 0, len(rows))
	for _, r := range rows {
		a := r.alert()
		if latest, err := m.latestNotification(ctx, a.ID); err == nil && latest != nil {
			a.Notification = latest
		}
		out = append(out, a)
	}
	return out, total, nil
}

func (m *PostgresAlertModel) Get(ctx context.Context, id int64) (*Alert, error) {
	var row alertRow
	if err := m.conn.QueryRowCtx(ctx, &row, alertSelect+" WHERE id = $1", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get alert: %w", err)
	}
	a := row.alert()
	if latest, err := m.latestNotification(ctx, id); err == nil && latest != nil {
		a.Notification = latest
	}
	return &a, nil
}

func (m *PostgresAlertModel) SetStatus(ctx context.Context, id int64, status string, at time.Time) error {
	var resolved any
	if status == AlertStatusResolved {
		resolved = at
	}
	const query = `UPDATE alerts SET status = $1, resolved_at = $2 WHERE id = $3`
	if _, err := m.conn.ExecCtx(ctx, query, status, resolved, id); err != nil {
		return fmt.Errorf("set alert status: %w", err)
	}
	return nil
}

func (m *PostgresAlertModel) InsertNotification(ctx context.Context, n AlertNotification) error {
	const query = `INSERT INTO alert_notifications (alert_id, channel_id, status, error_code, sent_at)
VALUES ($1, $2, $3, $4, $5)`
	if _, err := m.conn.ExecCtx(ctx, query, n.AlertID, n.ChannelID, n.Status, n.ErrorCode, n.SentAt); err != nil {
		return fmt.Errorf("insert alert notification: %w", err)
	}
	return nil
}

func (m *PostgresAlertModel) Notifications(ctx context.Context, alertID int64) ([]AlertNotification, error) {
	const query = `SELECT id, alert_id, channel_id, status, error_code, sent_at
FROM alert_notifications WHERE alert_id = $1 ORDER BY sent_at DESC`
	var rows []alertNotificationRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, alertID); err != nil {
		return nil, fmt.Errorf("list alert notifications: %w", err)
	}
	out := make([]AlertNotification, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.notification())
	}
	return out, nil
}

func (m *PostgresAlertModel) latestNotification(ctx context.Context, alertID int64) (*AlertNotification, error) {
	const query = `SELECT id, alert_id, channel_id, status, error_code, sent_at
FROM alert_notifications WHERE alert_id = $1 ORDER BY sent_at DESC LIMIT 1`
	var row alertNotificationRow
	if err := m.conn.QueryRowCtx(ctx, &row, query, alertID); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	n := row.notification()
	return &n, nil
}

type alertNotificationRow struct {
	Id        int64     `db:"id"`
	AlertId   int64     `db:"alert_id"`
	ChannelId int64     `db:"channel_id"`
	Status    string    `db:"status"`
	ErrorCode string    `db:"error_code"`
	SentAt    time.Time `db:"sent_at"`
}

func (r alertNotificationRow) notification() AlertNotification {
	return AlertNotification{ID: r.Id, AlertID: r.AlertId, ChannelID: r.ChannelId, Status: r.Status, ErrorCode: r.ErrorCode, SentAt: r.SentAt}
}

func (m *PostgresAlertModel) Recent(ctx context.Context, limit int) ([]Alert, error) {
	if limit <= 0 {
		limit = 10
	}
	query := alertSelect + " WHERE status = $1 ORDER BY last_seen_at DESC LIMIT $2"
	var rows []alertRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, AlertStatusActive, limit); err != nil {
		return nil, fmt.Errorf("recent alerts: %w", err)
	}
	out := make([]Alert, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.alert())
	}
	return out, nil
}

func (m *PostgresAlertModel) Counts(ctx context.Context) (int64, int64, int64, int64, error) {
	const query = `SELECT
  count(*) FILTER (WHERE status = 'active') AS active,
  count(*) FILTER (WHERE status = 'active' AND level = 'info') AS info,
  count(*) FILTER (WHERE status = 'active' AND level = 'warning') AS warning,
  count(*) FILTER (WHERE status = 'active' AND level = 'critical') AS critical
FROM alerts`
	var counts struct {
		Active   int64 `db:"active"`
		Info     int64 `db:"info"`
		Warning  int64 `db:"warning"`
		Critical int64 `db:"critical"`
	}
	if err := m.conn.QueryRowCtx(ctx, &counts, query); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("count alerts: %w", err)
	}
	return counts.Active, counts.Info, counts.Warning, counts.Critical, nil
}

// Trend 按 step 分桶返回各级别新增告警数；无数据返回空序列。
func (m *PostgresAlertModel) Trend(ctx context.Context, from, to time.Time, step time.Duration) ([]TrendPoint, []TrendPoint, []TrendPoint, error) {
	const query = `SELECT level, first_seen_at FROM alerts WHERE first_seen_at >= $1 AND first_seen_at <= $2`
	type row struct {
		Level       string    `db:"level"`
		FirstSeenAt time.Time `db:"first_seen_at"`
	}
	var rows []row
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, from, to); err != nil {
		return nil, nil, nil, fmt.Errorf("alert trend: %w", err)
	}
	if step <= 0 {
		step = time.Hour
	}
	buckets := int(to.Sub(from)/step) + 1
	if buckets < 0 {
		buckets = 0
	}
	makeSeries := func() []TrendPoint {
		out := make([]TrendPoint, buckets)
		for i := range out {
			out[i] = TrendPoint{TS: from.Add(time.Duration(i) * step)}
		}
		return out
	}
	info, warning, critical := makeSeries(), makeSeries(), makeSeries()
	bucketOf := func(t time.Time) int {
		idx := int(t.Sub(from) / step)
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}
		return idx
	}
	for _, r := range rows {
		i := bucketOf(r.FirstSeenAt)
		switch r.Level {
		case "info":
			info[i].Value++
		case "warning":
			warning[i].Value++
		case "critical":
			critical[i].Value++
		}
	}
	return info, warning, critical, nil
}
