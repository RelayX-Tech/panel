package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// PostgresNodeModel 读取 STO-002 依赖表（nodes / entry_groups / tunnel_groups）与 node_events。
// 写入 nodes 由 Agent 上报链路负责；STO-004 只读。
type PostgresNodeModel struct {
	conn sqlx.SqlConn
}

func NewNodeModel(conn sqlx.SqlConn) *PostgresNodeModel {
	return &PostgresNodeModel{conn: conn}
}

const nodeSelect = `SELECT n.id, n.name, n.type, n.group_id, n.region, n.ip, n.status,
       n.last_seen_at, n.agent_version, n.xray_version,
       n.cert_not_after, n.cpu_percent, n.mem_percent, n.disk_percent, n.load1,
       n.tcp_connections, n.net_in_bps, n.net_out_bps, n.xray_state, n.xray_restarts,
       n.xray_pid, n.xray_cpu_percent, n.xray_mem_bytes, n.xray_inbounds, n.xray_outbounds,
       n.nics, coalesce(eg.name, tg.name, '') AS group_name
FROM nodes n
LEFT JOIN entry_groups eg ON n.type = 'entry' AND eg.id = n.group_id
LEFT JOIN tunnel_groups tg ON n.type = 'tunnel' AND tg.id = n.group_id`

type nodeRow struct {
	Id             string     `db:"id"`
	Name           string     `db:"name"`
	Type           string     `db:"type"`
	GroupId        string     `db:"group_id"`
	Region         string     `db:"region"`
	Ip             string     `db:"ip"`
	Status         string     `db:"status"`
	LastSeenAt     *time.Time `db:"last_seen_at"`
	AgentVersion   string     `db:"agent_version"`
	XrayVersion    string     `db:"xray_version"`
	CertNotAfter   *time.Time `db:"cert_not_after"`
	CpuPercent     float64    `db:"cpu_percent"`
	MemPercent     float64    `db:"mem_percent"`
	DiskPercent    float64    `db:"disk_percent"`
	Load1          float64    `db:"load1"`
	TcpConnections int        `db:"tcp_connections"`
	NetInBps       float64    `db:"net_in_bps"`
	NetOutBps      float64    `db:"net_out_bps"`
	XrayState      string     `db:"xray_state"`
	XrayRestarts   int        `db:"xray_restarts"`
	XrayPid        int        `db:"xray_pid"`
	XrayCpuPercent float64    `db:"xray_cpu_percent"`
	XrayMemBytes   int64      `db:"xray_mem_bytes"`
	XrayInbounds   int        `db:"xray_inbounds"`
	XrayOutbounds  int        `db:"xray_outbounds"`
	Nics           []byte     `db:"nics"`
	GroupName      string     `db:"group_name"`
}

func (r nodeRow) summary() NodeSummary {
	return NodeSummary{
		ID: r.Id, Name: r.Name, Type: r.Type, GroupID: r.GroupId, GroupName: r.GroupName,
		Region: r.Region, IP: r.Ip, Status: r.Status, CPUPercent: r.CpuPercent,
		MemPercent: r.MemPercent, DiskPercent: r.DiskPercent, Load1: r.Load1,
		TCPConnections: r.TcpConnections, NetInBps: r.NetInBps, NetOutBps: r.NetOutBps,
		XrayState: r.XrayState, Restarts: r.XrayRestarts, LastSeenAt: r.LastSeenAt,
		AgentVersion: r.AgentVersion,
	}
}

func (m *PostgresNodeModel) List(ctx context.Context, f NodeFilter) ([]NodeSummary, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	var where []string
	var args []any
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if f.Type != "" {
		add("n.type = $%d", f.Type)
	}
	if f.Status != "" && f.Status != "upgrading" && f.Status != "pending_upgrade" {
		add("n.status = $%d", f.Status)
	}
	if f.GroupID != "" {
		add("n.group_id = $%d", f.GroupID)
	}
	if f.Region != "" {
		add("n.region = $%d", f.Region)
	}
	if f.Query != "" {
		args = append(args, "%"+f.Query+"%")
		where = append(where, fmt.Sprintf("(n.name ILIKE $%d OR n.ip ILIKE $%d)", len(args), len(args)))
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	orderSQL := nodeOrder(f.Sort)

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM nodes n"+whereSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count nodes: %w", err)
	}

	query := nodeSelect + whereSQL + orderSQL + fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	var rows []nodeRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list nodes: %w", err)
	}
	out := make([]NodeSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.summary())
	}
	return out, total, nil
}

func nodeOrder(sort string) string {
	switch sort {
	case "status":
		return " ORDER BY n.status ASC, n.id ASC"
	case "cpu":
		return " ORDER BY n.cpu_percent DESC, n.id ASC"
	case "mem":
		return " ORDER BY n.mem_percent DESC, n.id ASC"
	case "lastSeen":
		return " ORDER BY n.last_seen_at DESC NULLS LAST, n.id ASC"
	default:
		return " ORDER BY n.id ASC"
	}
}

func (m *PostgresNodeModel) Get(ctx context.Context, id string) (*NodeDetail, error) {
	var row nodeRow
	if err := m.conn.QueryRowCtx(ctx, &row, nodeSelect+" WHERE n.id = $1", id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get node: %w", err)
	}
	detail := &NodeDetail{
		NodeSummary:         row.summary(),
		XrayVersion:         row.XrayVersion,
		CertificateNotAfter: row.CertNotAfter,
		NICs:                []NodeNIC{},
		Xray: NodeXray{
			PID: row.XrayPid, CPUPercent: row.XrayCpuPercent, MemBytes: row.XrayMemBytes,
			Restarts: row.XrayRestarts, Inbounds: row.XrayInbounds, Outbounds: row.XrayOutbounds,
		},
	}
	if len(row.Nics) > 0 {
		_ = json.Unmarshal(row.Nics, &detail.NICs)
	}
	return detail, nil
}

func (m *PostgresNodeModel) Events(ctx context.Context, nodeID string, f EventFilter) ([]NodeEvent, int64, error) {
	page, pageSize := normalizePage(f.Page, f.PageSize)
	where := []string{"node_id = $1", "occurred_at >= now() - interval '30 days'"}
	args := []any{nodeID}
	if f.Type != "" {
		args = append(args, f.Type)
		where = append(where, fmt.Sprintf("type = $%d", len(args)))
	}
	if f.From != nil {
		args = append(args, *f.From)
		where = append(where, fmt.Sprintf("occurred_at >= $%d", len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		where = append(where, fmt.Sprintf("occurred_at <= $%d", len(args)))
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT count(*) FROM node_events"+whereSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count node events: %w", err)
	}
	query := "SELECT id, node_id, type, level, message, metadata, occurred_at FROM node_events" +
		whereSQL + fmt.Sprintf(" ORDER BY occurred_at DESC LIMIT %d OFFSET %d", pageSize, (page-1)*pageSize)
	type eventRow struct {
		Id         int64     `db:"id"`
		NodeId     string    `db:"node_id"`
		Type       string    `db:"type"`
		Level      string    `db:"level"`
		Message    string    `db:"message"`
		Metadata   []byte    `db:"metadata"`
		OccurredAt time.Time `db:"occurred_at"`
	}
	var rows []eventRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list node events: %w", err)
	}
	out := make([]NodeEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, NodeEvent{
			ID: r.Id, NodeID: r.NodeId, Type: r.Type, Level: r.Level,
			Message: r.Message, Metadata: json.RawMessage(r.Metadata), OccurredAt: r.OccurredAt,
		})
	}
	return out, total, nil
}

func (m *PostgresNodeModel) InsertEvent(ctx context.Context, e *NodeEvent) error {
	metadata := e.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	const query = `INSERT INTO node_events (node_id, type, level, message, metadata, occurred_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6) RETURNING id`
	if err := m.conn.QueryRowCtx(ctx, &e.ID, query, e.NodeID, e.Type, e.Level, e.Message, []byte(metadata), e.OccurredAt); err != nil {
		return fmt.Errorf("insert node event: %w", err)
	}
	return nil
}

func (m *PostgresNodeModel) CountsByTypeStatus(ctx context.Context, onlineSince time.Time) (int64, int64, int64, int64, error) {
	const query = `SELECT
  count(*) FILTER (WHERE type = 'entry' AND status = 'online' AND last_seen_at >= $1) AS entry_online,
  count(*) FILTER (WHERE type = 'entry') AS entry_total,
  count(*) FILTER (WHERE type = 'tunnel' AND status = 'online' AND last_seen_at >= $1) AS tunnel_online,
  count(*) FILTER (WHERE type = 'tunnel') AS tunnel_total
FROM nodes`
	var counts struct {
		EntryOnline  int64 `db:"entry_online"`
		EntryTotal   int64 `db:"entry_total"`
		TunnelOnline int64 `db:"tunnel_online"`
		TunnelTotal  int64 `db:"tunnel_total"`
	}
	if err := m.conn.QueryRowCtx(ctx, &counts, query, onlineSince); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("count nodes: %w", err)
	}
	return counts.EntryOnline, counts.EntryTotal, counts.TunnelOnline, counts.TunnelTotal, nil
}

// GroupNodes 按组返回节点健康；用于监控总览。
func (m *PostgresNodeModel) GroupNodes(ctx context.Context) ([]NodeHealthGroup, error) {
	const query = `SELECT n.type, n.group_id, coalesce(eg.name, tg.name, ''), n.id, n.name, n.ip, n.status, n.cpu_percent, n.mem_percent, n.xray_state
FROM nodes n
LEFT JOIN entry_groups eg ON n.type = 'entry' AND eg.id = n.group_id
LEFT JOIN tunnel_groups tg ON n.type = 'tunnel' AND tg.id = n.group_id
ORDER BY n.type, n.group_id, n.name`
	type row struct {
		Type       string  `db:"type"`
		GroupId    string  `db:"group_id"`
		GroupName  string  `db:"group_name"`
		Id         string  `db:"id"`
		Name       string  `db:"name"`
		Ip         string  `db:"ip"`
		Status     string  `db:"status"`
		CpuPercent float64 `db:"cpu_percent"`
		MemPercent float64 `db:"mem_percent"`
		XrayState  string  `db:"xray_state"`
	}
	var rows []row
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("group nodes: %w", err)
	}
	index := make(map[string]int)
	var out []NodeHealthGroup
	for _, r := range rows {
		key := r.Type + "/" + r.GroupId
		idx, ok := index[key]
		if !ok {
			idx = len(out)
			index[key] = idx
			out = append(out, NodeHealthGroup{GroupType: r.Type, GroupID: r.GroupId, GroupName: r.GroupName})
		}
		out[idx].Nodes = append(out[idx].Nodes, NodeHealthNode{
			NodeID: r.Id, NodeName: r.Name, IP: r.Ip, Status: r.Status,
			CPUPercent: r.CpuPercent, MemPercent: r.MemPercent, XrayState: r.XrayState,
		})
	}
	return out, nil
}

func (m *PostgresNodeModel) PortGroupStatusCounts(ctx context.Context) ([]StatusCount, error) {
	const query = `SELECT status, count(*)::bigint AS count FROM port_groups GROUP BY status ORDER BY status ASC`
	var rows []struct {
		Status string `db:"status"`
		Count  int64  `db:"count"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("port group status counts: %w", err)
	}
	out := make([]StatusCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, StatusCount{Status: r.Status, Count: r.Count})
	}
	return out, nil
}

func (m *PostgresNodeModel) XrayAbnormal(ctx context.Context) (int64, error) {
	var n int64
	const query = `SELECT count(*) FROM nodes WHERE xray_state NOT IN ('running', 'unknown', '')`
	if err := m.conn.QueryRowCtx(ctx, &n, query); err != nil {
		return 0, fmt.Errorf("count xray abnormal: %w", err)
	}
	return n, nil
}

func (m *PostgresNodeModel) ComponentVersions(ctx context.Context) (ComponentVersions, error) {
	const query = `SELECT
  coalesce(min(agent_version) FILTER (WHERE agent_version <> ''), '') AS agent_min,
  coalesce(max(agent_version) FILTER (WHERE agent_version <> ''), '') AS agent_max,
  coalesce(max(xray_version) FILTER (WHERE xray_version <> ''), '') AS xray
FROM nodes`
	var row struct {
		AgentMin string `db:"agent_min"`
		AgentMax string `db:"agent_max"`
		Xray     string `db:"xray"`
	}
	if err := m.conn.QueryRowCtx(ctx, &row, query); err != nil {
		return ComponentVersions{}, fmt.Errorf("component versions: %w", err)
	}
	return ComponentVersions{AgentMin: row.AgentMin, AgentMax: row.AgentMax, Xray: row.Xray}, nil
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
