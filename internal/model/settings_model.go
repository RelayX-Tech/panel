package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresSettingModel struct {
	conn sqlx.SqlConn
}

func NewSettingModel(conn sqlx.SqlConn) *PostgresSettingModel {
	return &PostgresSettingModel{conn: conn}
}

func (m *PostgresSettingModel) GetJSON(ctx context.Context, key string) ([]byte, bool, error) {
	var raw string
	const query = `SELECT value_json::text FROM panel_settings WHERE key = $1`
	if err := m.conn.QueryRowCtx(ctx, &raw, query, key); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get setting %s: %w", key, err)
	}
	return []byte(raw), true, nil
}

func (m *PostgresSettingModel) SetJSON(ctx context.Context, key string, value []byte) error {
	const query = `INSERT INTO panel_settings (key, value_json, updated_at) VALUES ($1, $2::jsonb, now())
ON CONFLICT (key) DO UPDATE SET value_json = excluded.value_json, updated_at = now()`
	if _, err := m.conn.ExecCtx(ctx, query, key, value); err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

// ---- 订阅 ----

type PostgresSubscriptionModel struct {
	conn sqlx.SqlConn
}

func NewSubscriptionModel(conn sqlx.SqlConn) *PostgresSubscriptionModel {
	return &PostgresSubscriptionModel{conn: conn}
}

const subscriptionHostSelect = `SELECT h.id, h.host, h.entry_group_id, coalesce(eg.name, '') AS entry_group_name,
       h.tls_mode, h.cert_not_after, h.created_at
FROM subscription_hosts h
LEFT JOIN entry_groups eg ON eg.id = h.entry_group_id`

type subscriptionHostRow struct {
	Id             int64      `db:"id"`
	Host           string     `db:"host"`
	EntryGroupId   string     `db:"entry_group_id"`
	EntryGroupName string     `db:"entry_group_name"`
	TlsMode        string     `db:"tls_mode"`
	CertNotAfter   *time.Time `db:"cert_not_after"`
	CreatedAt      time.Time  `db:"created_at"`
}

func (r subscriptionHostRow) host() SubscriptionHost {
	status := "pending"
	if r.CertNotAfter != nil {
		if r.CertNotAfter.After(time.Now()) {
			status = "valid"
		} else {
			status = "expired"
		}
	}
	return SubscriptionHost{
		ID: r.Id, Host: r.Host, EntryGroupID: r.EntryGroupId, EntryGroupName: r.EntryGroupName,
		TLSMode: r.TlsMode, TLSStatus: status, CertNotAfter: r.CertNotAfter, CreatedAt: r.CreatedAt,
	}
}

func (m *PostgresSubscriptionModel) ListHosts(ctx context.Context, page, pageSize int) ([]SubscriptionHost, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM subscription_hosts"); err != nil {
		return nil, 0, fmt.Errorf("count subscription hosts: %w", err)
	}
	query := subscriptionHostSelect + fmt.Sprintf(" ORDER BY h.id ASC LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	var rows []subscriptionHostRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, 0, fmt.Errorf("list subscription hosts: %w", err)
	}
	out := make([]SubscriptionHost, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.host())
	}
	return out, total, nil
}

func (m *PostgresSubscriptionModel) GetHost(ctx context.Context, id int64) (*SubscriptionHost, error) {
	var row subscriptionHostRow
	if err := m.conn.QueryRowCtx(ctx, &row, subscriptionHostSelect+" WHERE h.id = $1", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get subscription host: %w", err)
	}
	h := row.host()
	return &h, nil
}

func (m *PostgresSubscriptionModel) GetHostByName(ctx context.Context, host string) (*SubscriptionHost, error) {
	var row subscriptionHostRow
	if err := m.conn.QueryRowCtx(ctx, &row, subscriptionHostSelect+" WHERE h.host = $1", host); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get subscription host by name: %w", err)
	}
	h := row.host()
	return &h, nil
}

func (m *PostgresSubscriptionModel) CreateHost(ctx context.Context, h SubscriptionHost) (*SubscriptionHost, error) {
	const query = `INSERT INTO subscription_hosts (host, entry_group_id, tls_mode) VALUES ($1, $2, $3) RETURNING id`
	if err := m.conn.QueryRowCtx(ctx, &h.ID, query, h.Host, h.EntryGroupID, h.TLSMode); err != nil {
		return nil, fmt.Errorf("create subscription host: %w", err)
	}
	return m.GetHost(ctx, h.ID)
}

func (m *PostgresSubscriptionModel) UpdateHost(ctx context.Context, id int64, entryGroupID, tlsMode *string) (*SubscriptionHost, error) {
	current, err := m.GetHost(ctx, id)
	if err != nil {
		return nil, err
	}
	if entryGroupID != nil {
		current.EntryGroupID = *entryGroupID
	}
	if tlsMode != nil {
		current.TLSMode = *tlsMode
	}
	const query = `UPDATE subscription_hosts SET entry_group_id = $1, tls_mode = $2, updated_at = now() WHERE id = $3`
	if _, err := m.conn.ExecCtx(ctx, query, current.EntryGroupID, current.TLSMode, id); err != nil {
		return nil, fmt.Errorf("update subscription host: %w", err)
	}
	return m.GetHost(ctx, id)
}

func (m *PostgresSubscriptionModel) DeleteHost(ctx context.Context, id int64) error {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM subscription_hosts WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete subscription host: %w", err)
	}
	return nil
}

func (m *PostgresSubscriptionModel) ListFormats(ctx context.Context) ([]SubscriptionFormat, error) {
	const query = `SELECT key, label, enabled, sort_order FROM subscription_formats ORDER BY sort_order ASC, key ASC`
	var rows []struct {
		Key       string `db:"key"`
		Label     string `db:"label"`
		Enabled   bool   `db:"enabled"`
		SortOrder int    `db:"sort_order"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("list subscription formats: %w", err)
	}
	out := make([]SubscriptionFormat, 0, len(rows))
	for _, r := range rows {
		out = append(out, SubscriptionFormat{
			Key: r.Key, Label: r.Label, Enabled: r.Enabled, SortOrder: r.SortOrder,
			Source: "db", CapabilitySupported: true,
		})
	}
	return out, nil
}

func (m *PostgresSubscriptionModel) SetFormatEnabled(ctx context.Context, key string, enabled bool, sortOrder int) error {
	const query = `INSERT INTO subscription_formats (key, label, enabled, sort_order, updated_at)
VALUES ($1, $1, $2, $3, now())
ON CONFLICT (key) DO UPDATE SET enabled = excluded.enabled, sort_order = excluded.sort_order, updated_at = now()`
	if _, err := m.conn.ExecCtx(ctx, query, key, enabled, sortOrder); err != nil {
		return fmt.Errorf("set subscription format: %w", err)
	}
	return nil
}
