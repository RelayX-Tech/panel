package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresUpgradeModel struct {
	conn sqlx.SqlConn
}

func NewUpgradeModel(conn sqlx.SqlConn) *PostgresUpgradeModel {
	return &PostgresUpgradeModel{conn: conn}
}

func (m *PostgresUpgradeModel) CreateTask(ctx context.Context, t *UpgradeTask) error {
	const query = `INSERT INTO upgrade_tasks
    (id, scope, target_version, from_version, batch_id, status, error_code, error_message,
     created_by, request_id, created_at, started_at, finished_at)
VALUES ($1::uuid, $2, $3, $4, nullif($5, '')::uuid, $6, $7, $8, $9, $10, $11, $12, $13)`
	if _, err := m.conn.ExecCtx(ctx, query, t.ID, t.Scope, t.TargetVersion, t.FromVersion, t.BatchID,
		t.Status, t.ErrorCode, t.ErrorMessage, t.CreatedBy, t.RequestID, t.CreatedAt, timeArg(t.StartedAt), timeArg(t.FinishedAt)); err != nil {
		return fmt.Errorf("create upgrade task: %w", err)
	}
	return nil
}

const upgradeTaskSelect = `SELECT id::text AS id, scope, target_version, from_version, coalesce(batch_id::text, '') AS batch_id,
       status, error_code, error_message, created_by, request_id, created_at, started_at, finished_at
FROM upgrade_tasks`

type upgradeTaskRow struct {
	Id            string     `db:"id"`
	Scope         string     `db:"scope"`
	TargetVersion string     `db:"target_version"`
	FromVersion   string     `db:"from_version"`
	BatchId       string     `db:"batch_id"`
	Status        string     `db:"status"`
	ErrorCode     string     `db:"error_code"`
	ErrorMessage  string     `db:"error_message"`
	CreatedBy     string     `db:"created_by"`
	RequestId     string     `db:"request_id"`
	CreatedAt     time.Time  `db:"created_at"`
	StartedAt     *time.Time `db:"started_at"`
	FinishedAt    *time.Time `db:"finished_at"`
}

func (r upgradeTaskRow) task() UpgradeTask {
	return UpgradeTask{
		ID: r.Id, Scope: r.Scope, TargetVersion: r.TargetVersion, FromVersion: r.FromVersion,
		BatchID: r.BatchId, Status: r.Status, ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage,
		CreatedBy: r.CreatedBy, RequestID: r.RequestId, CreatedAt: r.CreatedAt,
		StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
	}
}

func (m *PostgresUpgradeModel) GetTask(ctx context.Context, id string) (*UpgradeTask, error) {
	var row upgradeTaskRow
	if err := m.conn.QueryRowCtx(ctx, &row, upgradeTaskSelect+" WHERE id = $1::uuid", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get upgrade task: %w", err)
	}
	task := row.task()
	nodes, err := m.ListTaskNodes(ctx, id)
	if err != nil {
		return nil, err
	}
	task.Nodes = nodes
	return &task, nil
}

func (m *PostgresUpgradeModel) ListTasks(ctx context.Context, f UpgradeTaskFilter) ([]UpgradeTask, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	var where []string
	var args []any
	if f.Scope != "" {
		args = append(args, f.Scope)
		where = append(where, fmt.Sprintf("scope = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM upgrade_tasks"+whereSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count upgrade tasks: %w", err)
	}
	query := upgradeTaskSelect + whereSQL + fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	var rows []upgradeTaskRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list upgrade tasks: %w", err)
	}
	out := make([]UpgradeTask, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.task())
	}
	return out, total, nil
}

func (m *PostgresUpgradeModel) UpdateTaskStatus(ctx context.Context, id, status, errorCode, errorMessage string, startedAt, finishedAt *time.Time) error {
	const query = `UPDATE upgrade_tasks
SET status = $1, error_code = $2, error_message = $3,
    started_at = coalesce($4, started_at), finished_at = coalesce($5, finished_at)
WHERE id = $6::uuid`
	if _, err := m.conn.ExecCtx(ctx, query, status, errorCode, errorMessage, timeArg(startedAt), timeArg(finishedAt), id); err != nil {
		return fmt.Errorf("update upgrade task status: %w", err)
	}
	return nil
}

func (m *PostgresUpgradeModel) UpsertTaskNode(ctx context.Context, n *UpgradeTaskNode) error {
	const query = `INSERT INTO upgrade_task_nodes (task_id, node_id, from_version, to_version, state, error_code, error_message, updated_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (task_id, node_id) DO UPDATE SET
    from_version = excluded.from_version, to_version = excluded.to_version, state = excluded.state,
    error_code = excluded.error_code, error_message = excluded.error_message, updated_at = excluded.updated_at`
	if _, err := m.conn.ExecCtx(ctx, query, n.TaskID, n.NodeID, n.FromVersion, n.ToVersion, n.State, n.ErrorCode, n.ErrorMessage, n.UpdatedAt); err != nil {
		return fmt.Errorf("upsert upgrade task node: %w", err)
	}
	return nil
}

func (m *PostgresUpgradeModel) ListTaskNodes(ctx context.Context, taskID string) ([]UpgradeTaskNode, error) {
	const query = `SELECT n.task_id::text AS task_id, n.node_id, coalesce(nd.name, n.node_id) AS node_name,
       n.from_version, n.to_version, n.state, n.error_code, n.error_message, n.updated_at
FROM upgrade_task_nodes n
LEFT JOIN nodes nd ON nd.id = n.node_id
WHERE n.task_id = $1::uuid ORDER BY n.node_id ASC`
	var rows []struct {
		TaskId       string    `db:"task_id"`
		NodeId       string    `db:"node_id"`
		NodeName     string    `db:"node_name"`
		FromVersion  string    `db:"from_version"`
		ToVersion    string    `db:"to_version"`
		State        string    `db:"state"`
		ErrorCode    string    `db:"error_code"`
		ErrorMessage string    `db:"error_message"`
		UpdatedAt    time.Time `db:"updated_at"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, taskID); err != nil {
		return nil, fmt.Errorf("list upgrade task nodes: %w", err)
	}
	out := make([]UpgradeTaskNode, 0, len(rows))
	for _, r := range rows {
		out = append(out, UpgradeTaskNode{
			TaskID: r.TaskId, NodeID: r.NodeId, NodeName: r.NodeName, FromVersion: r.FromVersion,
			ToVersion: r.ToVersion, State: r.State, ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (m *PostgresUpgradeModel) ActiveTask(ctx context.Context) (*UpgradeTask, error) {
	var row upgradeTaskRow
	query := upgradeTaskSelect + ` WHERE status IN ('queued','running','rolling_back') ORDER BY created_at DESC LIMIT 1`
	if err := m.conn.QueryRowCtx(ctx, &row, query); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("active upgrade task: %w", err)
	}
	task := row.task()
	nodes, err := m.ListTaskNodes(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	task.Nodes = nodes
	return &task, nil
}

// AcquireLock 使用单行锁表实现单飞：过期或同一持有者可覆盖。
func (m *PostgresUpgradeModel) AcquireLock(ctx context.Context, holder, scope string, now time.Time, ttl time.Duration) (bool, error) {
	const query = `INSERT INTO upgrade_lock (id, holder, scope, acquired_at, heartbeat_at, expires_at)
VALUES (1, $1, $2, $3, $3, $4)
ON CONFLICT (id) DO UPDATE SET
    holder = excluded.holder, scope = excluded.scope, acquired_at = excluded.acquired_at,
    heartbeat_at = excluded.heartbeat_at, expires_at = excluded.expires_at
WHERE upgrade_lock.expires_at <= $3 OR upgrade_lock.holder = excluded.holder
RETURNING holder`
	var got string
	if err := m.conn.QueryRowCtx(ctx, &got, query, holder, scope, now, now.Add(ttl)); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("acquire upgrade lock: %w", err)
	}
	return got == holder, nil
}

func (m *PostgresUpgradeModel) GetLock(ctx context.Context) (*UpgradeLock, error) {
	var row struct {
		Holder      string    `db:"holder"`
		Scope       string    `db:"scope"`
		AcquiredAt  time.Time `db:"acquired_at"`
		HeartbeatAt time.Time `db:"heartbeat_at"`
		ExpiresAt   time.Time `db:"expires_at"`
	}
	const query = `SELECT holder, scope, acquired_at, heartbeat_at, expires_at FROM upgrade_lock WHERE id = 1`
	if err := m.conn.QueryRowCtx(ctx, &row, query); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get upgrade lock: %w", err)
	}
	return &UpgradeLock{Holder: row.Holder, Scope: row.Scope, AcquiredAt: row.AcquiredAt, HeartbeatAt: row.HeartbeatAt, ExpiresAt: row.ExpiresAt}, nil
}

func (m *PostgresUpgradeModel) RenewLock(ctx context.Context, holder string, now time.Time, ttl time.Duration) (bool, error) {
	const query = `UPDATE upgrade_lock SET heartbeat_at = $1, expires_at = $2 WHERE id = 1 AND holder = $3 RETURNING holder`
	var got string
	if err := m.conn.QueryRowCtx(ctx, &got, query, now, now.Add(ttl), holder); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("renew upgrade lock: %w", err)
	}
	return true, nil
}

func (m *PostgresUpgradeModel) ReleaseLock(ctx context.Context, holder string) error {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM upgrade_lock WHERE id = 1 AND holder = $1`, holder); err != nil {
		return fmt.Errorf("release upgrade lock: %w", err)
	}
	return nil
}

func (m *PostgresUpgradeModel) UpsertTarget(ctx context.Context, t UpgradeTarget) error {
	const query = `INSERT INTO node_upgrade_targets (node_id, target_version, batch_id, created_at, expires_at)
VALUES ($1, $2, $3::uuid, $4, $5)
ON CONFLICT (node_id) DO UPDATE SET
    target_version = excluded.target_version, batch_id = excluded.batch_id,
    created_at = excluded.created_at, expires_at = excluded.expires_at`
	if _, err := m.conn.ExecCtx(ctx, query, t.NodeID, t.TargetVersion, t.BatchID, t.CreatedAt, t.ExpiresAt); err != nil {
		return fmt.Errorf("upsert upgrade target: %w", err)
	}
	return nil
}

func (m *PostgresUpgradeModel) ListTargets(ctx context.Context) ([]UpgradeTarget, error) {
	const query = `SELECT t.node_id, t.target_version, t.batch_id::text AS batch_id, t.created_at, t.expires_at,
       coalesce(nd.name, t.node_id) AS node_name
FROM node_upgrade_targets t LEFT JOIN nodes nd ON nd.id = t.node_id
ORDER BY t.expires_at ASC`
	var rows []struct {
		NodeId        string    `db:"node_id"`
		TargetVersion string    `db:"target_version"`
		BatchId       string    `db:"batch_id"`
		CreatedAt     time.Time `db:"created_at"`
		ExpiresAt     time.Time `db:"expires_at"`
		NodeName      string    `db:"node_name"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("list upgrade targets: %w", err)
	}
	out := make([]UpgradeTarget, 0, len(rows))
	for _, r := range rows {
		out = append(out, UpgradeTarget{NodeID: r.NodeId, NodeName: r.NodeName, TargetVersion: r.TargetVersion, BatchID: r.BatchId, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt})
	}
	return out, nil
}

func (m *PostgresUpgradeModel) DeleteTarget(ctx context.Context, nodeID string) error {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM node_upgrade_targets WHERE node_id = $1`, nodeID); err != nil {
		return fmt.Errorf("delete upgrade target: %w", err)
	}
	return nil
}

func (m *PostgresUpgradeModel) DeleteTargetsByBatch(ctx context.Context, batchID string) error {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM node_upgrade_targets WHERE batch_id = $1::uuid`, batchID); err != nil {
		return fmt.Errorf("delete upgrade targets by batch: %w", err)
	}
	return nil
}
