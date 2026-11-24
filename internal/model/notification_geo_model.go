package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresNotificationModel struct {
	conn sqlx.SqlConn
}

func NewNotificationModel(conn sqlx.SqlConn) *PostgresNotificationModel {
	return &PostgresNotificationModel{conn: conn}
}

const notificationSelect = `SELECT id, name, type, target_encrypted, target_masked, min_level, enabled, last_sent_at, created_at, updated_at
FROM notification_channels`

type notificationRow struct {
	Id              int64      `db:"id"`
	Name            string     `db:"name"`
	Type            string     `db:"type"`
	TargetEncrypted []byte     `db:"target_encrypted"`
	TargetMasked    string     `db:"target_masked"`
	MinLevel        string     `db:"min_level"`
	Enabled         bool       `db:"enabled"`
	LastSentAt      *time.Time `db:"last_sent_at"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

func (r notificationRow) channel() NotificationChannel {
	return NotificationChannel{
		ID: r.Id, Name: r.Name, Type: r.Type, TargetEncrypted: r.TargetEncrypted,
		TargetMasked: r.TargetMasked, MinLevel: r.MinLevel, Enabled: r.Enabled,
		LastSentAt: r.LastSentAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (m *PostgresNotificationModel) List(ctx context.Context) ([]NotificationChannel, error) {
	var rows []notificationRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, notificationSelect+" ORDER BY id ASC"); err != nil {
		return nil, fmt.Errorf("list notification channels: %w", err)
	}
	out := make([]NotificationChannel, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.channel())
	}
	return out, nil
}

func (m *PostgresNotificationModel) Get(ctx context.Context, id int64) (*NotificationChannel, error) {
	var row notificationRow
	if err := m.conn.QueryRowCtx(ctx, &row, notificationSelect+" WHERE id = $1", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get notification channel: %w", err)
	}
	c := row.channel()
	return &c, nil
}

func (m *PostgresNotificationModel) Create(ctx context.Context, c NotificationChannel) (*NotificationChannel, error) {
	const query = `INSERT INTO notification_channels (name, type, target_encrypted, target_masked, min_level, enabled)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	if err := m.conn.QueryRowCtx(ctx, &c.ID, query, c.Name, c.Type, c.TargetEncrypted, c.TargetMasked, c.MinLevel, c.Enabled); err != nil {
		return nil, fmt.Errorf("create notification channel: %w", err)
	}
	return m.Get(ctx, c.ID)
}

func (m *PostgresNotificationModel) Update(ctx context.Context, id int64, p NotificationPatch) (*NotificationChannel, error) {
	current, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		current.Name = *p.Name
	}
	if p.TargetEncrypted != nil {
		current.TargetEncrypted = p.TargetEncrypted
	}
	if p.TargetMasked != nil {
		current.TargetMasked = *p.TargetMasked
	}
	if p.MinLevel != nil {
		current.MinLevel = *p.MinLevel
	}
	if p.Enabled != nil {
		current.Enabled = *p.Enabled
	}
	const query = `UPDATE notification_channels SET name = $1, target_encrypted = $2, target_masked = $3,
       min_level = $4, enabled = $5, updated_at = now() WHERE id = $6`
	if _, err := m.conn.ExecCtx(ctx, query, current.Name, current.TargetEncrypted, current.TargetMasked, current.MinLevel, current.Enabled, id); err != nil {
		return nil, fmt.Errorf("update notification channel: %w", err)
	}
	return m.Get(ctx, id)
}

func (m *PostgresNotificationModel) Delete(ctx context.Context, id int64) error {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM notification_channels WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete notification channel: %w", err)
	}
	return nil
}

func (m *PostgresNotificationModel) MarkSent(ctx context.Context, id int64, at time.Time) error {
	if _, err := m.conn.ExecCtx(ctx, `UPDATE notification_channels SET last_sent_at = $1, updated_at = now() WHERE id = $2`, at, id); err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

// ---- Geo ----

type PostgresGeoModel struct {
	conn sqlx.SqlConn
}

func NewGeoModel(conn sqlx.SqlConn) *PostgresGeoModel {
	return &PostgresGeoModel{conn: conn}
}

const geoSelect = `SELECT id, kind, name, version, size_bytes, checksum, categories_count, status, path, updated_at FROM geo_files`

type geoRow struct {
	Id              int64     `db:"id"`
	Kind            string    `db:"kind"`
	Name            string    `db:"name"`
	Version         string    `db:"version"`
	SizeBytes       int64     `db:"size_bytes"`
	Checksum        string    `db:"checksum"`
	CategoriesCount int       `db:"categories_count"`
	Status          string    `db:"status"`
	Path            string    `db:"path"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func (r geoRow) file() GeoFile {
	return GeoFile{ID: r.Id, Kind: r.Kind, Name: r.Name, Version: r.Version, SizeBytes: r.SizeBytes,
		Checksum: r.Checksum, CategoriesCount: r.CategoriesCount, Status: r.Status, Path: r.Path, UpdatedAt: r.UpdatedAt}
}

func (m *PostgresGeoModel) List(ctx context.Context, kind string, page, pageSize int) ([]GeoFile, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	args := []any{}
	where := ""
	if kind != "" {
		args = append(args, kind)
		where = " WHERE kind = $1"
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM geo_files"+where, args...); err != nil {
		return nil, 0, fmt.Errorf("count geo files: %w", err)
	}
	query := geoSelect + where + fmt.Sprintf(" ORDER BY kind ASC, name ASC LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	var rows []geoRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list geo files: %w", err)
	}
	out := make([]GeoFile, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.file())
	}
	return out, total, nil
}

func (m *PostgresGeoModel) Get(ctx context.Context, id int64) (*GeoFile, error) {
	var row geoRow
	if err := m.conn.QueryRowCtx(ctx, &row, geoSelect+" WHERE id = $1", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get geo file: %w", err)
	}
	f := row.file()
	return &f, nil
}

func (m *PostgresGeoModel) Upsert(ctx context.Context, f GeoFile) (*GeoFile, error) {
	const query = `INSERT INTO geo_files (kind, name, version, size_bytes, checksum, status, path)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (kind, name, version) DO UPDATE SET
    size_bytes = excluded.size_bytes, checksum = excluded.checksum,
    status = excluded.status, path = excluded.path, updated_at = now()
RETURNING id`
	if err := m.conn.QueryRowCtx(ctx, &f.ID, query, f.Kind, f.Name, f.Version, f.SizeBytes, f.Checksum, f.Status, f.Path); err != nil {
		return nil, fmt.Errorf("upsert geo file: %w", err)
	}
	return m.Get(ctx, f.ID)
}

func (m *PostgresGeoModel) ReplaceCategories(ctx context.Context, fileID int64, cats []GeoCategory) error {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM geo_file_categories WHERE file_id = $1`, fileID); err != nil {
		return fmt.Errorf("clear geo categories: %w", err)
	}
	for _, c := range cats {
		const query = `INSERT INTO geo_file_categories (file_id, category, entries_count) VALUES ($1, $2, $3)`
		if _, err := m.conn.ExecCtx(ctx, query, fileID, c.Category, c.EntriesCount); err != nil {
			return fmt.Errorf("insert geo category: %w", err)
		}
	}
	const update = `UPDATE geo_files SET categories_count = $1, updated_at = now() WHERE id = $2`
	if _, err := m.conn.ExecCtx(ctx, update, len(cats), fileID); err != nil {
		return fmt.Errorf("update geo categories count: %w", err)
	}
	return nil
}

func (m *PostgresGeoModel) Categories(ctx context.Context, fileID int64, page, pageSize int) ([]GeoCategory, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `SELECT count(*) FROM geo_file_categories WHERE file_id = $1`, fileID); err != nil {
		return nil, 0, fmt.Errorf("count geo categories: %w", err)
	}
	query := fmt.Sprintf(`SELECT category, entries_count FROM geo_file_categories WHERE file_id = $1
ORDER BY category ASC LIMIT %d OFFSET %d`, pageSize, (page-1)*pageSize)
	var rows []struct {
		Category     string `db:"category"`
		EntriesCount int    `db:"entries_count"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, fileID); err != nil {
		return nil, 0, fmt.Errorf("list geo categories: %w", err)
	}
	out := make([]GeoCategory, 0, len(rows))
	for _, r := range rows {
		out = append(out, GeoCategory{Category: r.Category, EntriesCount: r.EntriesCount})
	}
	return out, total, nil
}
