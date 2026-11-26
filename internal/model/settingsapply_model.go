package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// PostgresSettingsApplyModel 读写探测规格下发任务（design.md §5.3）。
type PostgresSettingsApplyModel struct {
	conn sqlx.SqlConn
}

func NewSettingsApplyModel(conn sqlx.SqlConn) *PostgresSettingsApplyModel {
	return &PostgresSettingsApplyModel{conn: conn}
}

func (m *PostgresSettingsApplyModel) CreateTask(ctx context.Context, t *SettingsApplyTask) error {
	const query = `INSERT INTO settings_apply_tasks (id, kind, state, created_by, request_id, created_at, updated_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $6)`
	if _, err := m.conn.ExecCtx(ctx, query, t.ID, t.Kind, t.State, t.CreatedBy, t.RequestID, t.CreatedAt); err != nil {
		return fmt.Errorf("create settings apply task: %w", err)
	}
	return nil
}

const settingsApplyTaskSelect = `SELECT id::text AS id, kind, state, created_by, request_id, created_at, updated_at
FROM settings_apply_tasks`

type settingsApplyTaskRow struct {
	Id        string    `db:"id"`
	Kind      string    `db:"kind"`
	State     string    `db:"state"`
	CreatedBy string    `db:"created_by"`
	RequestId string    `db:"request_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r settingsApplyTaskRow) task() SettingsApplyTask {
	return SettingsApplyTask{
		ID: r.Id, Kind: r.Kind, State: r.State, CreatedBy: r.CreatedBy,
		RequestID: r.RequestId, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (m *PostgresSettingsApplyModel) GetTask(ctx context.Context, id string) (*SettingsApplyTask, error) {
	var row settingsApplyTaskRow
	if err := m.conn.QueryRowCtx(ctx, &row, settingsApplyTaskSelect+" WHERE id = $1::uuid", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get settings apply task: %w", err)
	}
	task := row.task()
	nodes, err := m.ListNodes(ctx, id)
	if err != nil {
		return nil, err
	}
	task.Nodes = nodes
	return &task, nil
}

func (m *PostgresSettingsApplyModel) LatestTask(ctx context.Context, kind string) (*SettingsApplyTask, error) {
	var row settingsApplyTaskRow
	query := settingsApplyTaskSelect + ` WHERE kind = $1 ORDER BY created_at DESC LIMIT 1`
	if err := m.conn.QueryRowCtx(ctx, &row, query, kind); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("latest settings apply task: %w", err)
	}
	task := row.task()
	nodes, err := m.ListNodes(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	task.Nodes = nodes
	return &task, nil
}

func (m *PostgresSettingsApplyModel) UpsertNode(ctx context.Context, n *SettingsApplyNode) error {
	const query = `INSERT INTO settings_apply_task_nodes (task_id, node_id, state, version, elapsed_ms, error_code, updated_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
ON CONFLICT (task_id, node_id) DO UPDATE SET
    state = excluded.state, version = excluded.version, elapsed_ms = excluded.elapsed_ms,
    error_code = excluded.error_code, updated_at = excluded.updated_at`
	if _, err := m.conn.ExecCtx(ctx, query, n.TaskID, n.NodeID, n.State, n.Version, n.ElapsedMs, n.ErrorCode, n.UpdatedAt); err != nil {
		return fmt.Errorf("upsert settings apply node: %w", err)
	}
	return nil
}

func (m *PostgresSettingsApplyModel) ListNodes(ctx context.Context, taskID string) ([]SettingsApplyNode, error) {
	const query = `SELECT n.task_id::text AS task_id, n.node_id, coalesce(nd.name, n.node_id) AS node_name,
       n.state, n.version, n.elapsed_ms, n.error_code, n.updated_at
FROM settings_apply_task_nodes n
LEFT JOIN nodes nd ON nd.id = n.node_id
WHERE n.task_id = $1::uuid ORDER BY n.node_id ASC`
	var rows []struct {
		TaskId    string    `db:"task_id"`
		NodeId    string    `db:"node_id"`
		NodeName  string    `db:"node_name"`
		State     string    `db:"state"`
		Version   int64     `db:"version"`
		ElapsedMs int64     `db:"elapsed_ms"`
		ErrorCode string    `db:"error_code"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, taskID); err != nil {
		return nil, fmt.Errorf("list settings apply nodes: %w", err)
	}
	out := make([]SettingsApplyNode, 0, len(rows))
	for _, r := range rows {
		out = append(out, SettingsApplyNode{
			TaskID: r.TaskId, NodeID: r.NodeId, NodeName: r.NodeName, State: r.State,
			Version: r.Version, ElapsedMs: r.ElapsedMs, ErrorCode: r.ErrorCode, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}
