package model

import (
	"context"
	"encoding/json"
	"time"
)

// STO-004 领域类型（design.md §3.3 / §6.2）。logic 只依赖这些类型与接口。

const (
	AlertStatusActive   = "active"
	AlertStatusResolved = "resolved"

	UpgradeTaskQueued      = "queued"
	UpgradeTaskRunning     = "running"
	UpgradeTaskSucceeded   = "succeeded"
	UpgradeTaskFailed      = "failed"
	UpgradeTaskRollingBack = "rolling_back"
	UpgradeTaskRolledBack  = "rolled_back"
	UpgradeTaskAborted     = "aborted"

	NodeUpgradePending     = "pending"
	NodeUpgradeDownloading = "downloading"
	NodeUpgradeVerifying   = "verifying"
	NodeUpgradeApplying    = "applying"
	NodeUpgradeSucceeded   = "succeeded"
	NodeUpgradeFailed      = "failed"
	NodeUpgradeRolledBack  = "rolled_back"
)

// ---- 节点 ----

type NodeSummary struct {
	ID             string
	Name           string
	Type           string
	GroupID        string
	GroupName      string
	Region         string
	IP             string
	Status         string
	DisplayState   string
	CPUPercent     float64
	MemPercent     float64
	DiskPercent    float64
	Load1          float64
	TCPConnections int
	NetInBps       float64
	NetOutBps      float64
	XrayState      string
	Restarts       int
	LastSeenAt     *time.Time
	AgentVersion   string
}

type NodeNIC struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	MTU       int      `json:"mtu"`
	Up        bool     `json:"up"`
	Addresses []string `json:"addresses"`
}

type NodeXray struct {
	PID        int
	CPUPercent float64
	MemBytes   int64
	Restarts   int
	Inbounds   int
	Outbounds  int
}

type NodeProbeInfo struct {
	Type                string
	LatencyMs           int64
	Success             bool
	Failures            int
	ConsecutiveFailures int
	CheckedAt           *time.Time
}

type NodeDetail struct {
	NodeSummary
	XrayVersion         string
	CertificateNotAfter *time.Time
	NICs                []NodeNIC
	Xray                NodeXray
	Probe               NodeProbeInfo
}

type NodeFilter struct {
	Page     int
	PageSize int
	Type     string
	Status   string
	GroupID  string
	Region   string
	Query    string
	Sort     string
}

type NodeEvent struct {
	ID         int64
	NodeID     string
	Type       string
	Level      string
	Message    string
	Metadata   json.RawMessage
	OccurredAt time.Time
}

type EventFilter struct {
	Page     int
	PageSize int
	Type     string
	From     *time.Time
	To       *time.Time
}

type NodeModel interface {
	List(ctx context.Context, f NodeFilter) ([]NodeSummary, int64, error)
	Get(ctx context.Context, id string) (*NodeDetail, error)
	Events(ctx context.Context, nodeID string, f EventFilter) ([]NodeEvent, int64, error)
	InsertEvent(ctx context.Context, e *NodeEvent) error
	CountsByTypeStatus(ctx context.Context, onlineSince time.Time) (entryOnline, entryTotal, tunnelOnline, tunnelTotal int64, err error)
	GroupNodes(ctx context.Context) ([]NodeHealthGroup, error)
	PortGroupStatusCounts(ctx context.Context) ([]StatusCount, error)
	XrayAbnormal(ctx context.Context) (int64, error)
	ComponentVersions(ctx context.Context) (ComponentVersions, error)
}

// ComponentVersions 是节点上报的组件版本聚合（最低 / 最高）。
type ComponentVersions struct {
	AgentMin string
	AgentMax string
	Xray     string
}

type StatusCount struct {
	Status string
	Count  int64
}

type NodeHealthNode struct {
	NodeID     string
	NodeName   string
	IP         string
	Status     string
	CPUPercent float64
	MemPercent float64
	XrayState  string
}

type NodeHealthGroup struct {
	GroupType string
	GroupID   string
	GroupName string
	Nodes     []NodeHealthNode
}

// ---- 告警 ----

type AlertNotification struct {
	ID        int64
	AlertID   int64
	ChannelID int64
	Status    string
	ErrorCode string
	SentAt    time.Time
}

type Alert struct {
	ID           int64
	Type         string
	Level        string
	ObjectType   string
	ObjectID     string
	ObjectName   string
	Message      string
	Status       string
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	ResolvedAt   *time.Time
	Notification *AlertNotification
}

type AlertFilter struct {
	Page       int
	PageSize   int
	Level      string
	Status     string
	Type       string
	ObjectType string
	Query      string
	Sort       string
	From       *time.Time
	To         *time.Time
}

type AlertModel interface {
	Upsert(ctx context.Context, a Alert) (*Alert, error)
	List(ctx context.Context, f AlertFilter) ([]Alert, int64, error)
	Get(ctx context.Context, id int64) (*Alert, error)
	SetStatus(ctx context.Context, id int64, status string, at time.Time) error
	InsertNotification(ctx context.Context, n AlertNotification) error
	Notifications(ctx context.Context, alertID int64) ([]AlertNotification, error)
	Recent(ctx context.Context, limit int) ([]Alert, error)
	Counts(ctx context.Context) (active int64, info, warning, critical int64, err error)
	Trend(ctx context.Context, from, to time.Time, step time.Duration) (info, warning, critical []TrendPoint, err error)
}

type TrendPoint struct {
	TS    time.Time
	Value float64
}

// ---- 流量 ----

type TrafficSample struct {
	EntityType  string
	EntityID    string
	TS          time.Time
	UpBytes     int64
	DownBytes   int64
	Connections int
}

type TrafficPoint struct {
	TS          time.Time
	UpBytes     int64
	DownBytes   int64
	Connections int
}

type TrafficRow struct {
	ObjectType    string
	ObjectID      string
	ObjectName    string
	UpBytes       int64
	DownBytes     int64
	TotalBytes    int64
	Connections   int
	LastUpdatedAt time.Time
}

type TrafficModel interface {
	UpsertSample(ctx context.Context, s TrafficSample) error
	Series(ctx context.Context, entityType, entityID, metric string, from, to time.Time, step time.Duration) ([]TrafficPoint, error)
	Rows(ctx context.Context, entityType string, from, to time.Time, page, pageSize int, sort string) ([]TrafficRow, int64, error)
	Totals(ctx context.Context, from, to time.Time) (upBytes, downBytes int64, connections int, err error)
	StorageStats(ctx context.Context) (StorageStats, error)
}

// StorageStats 是数据与保留页的存储用量（无数据字段为 nil）。
type StorageStats struct {
	DatabaseSizeBytes         int64
	TrafficPartitionSizeBytes int64
	EarliestRecordAt          *time.Time
	LatestRecordAt            *time.Time
}

// ---- 客户端 ----

type ClientRow struct {
	ClientID       string
	Email          string
	PortID         string
	PortGroupID    string
	UpBytes        int64
	DownBytes      int64
	LastOnlineAt   *time.Time
	LastSubFetchAt *time.Time
}

type ClientFilter struct {
	Page        int
	PageSize    int
	PortGroupID string
	PortID      string
	Query       string
}

type ClientModel interface {
	Upsert(ctx context.Context, c ClientRow) error
	List(ctx context.Context, f ClientFilter) ([]ClientRow, int64, error)
	OnlineSince(ctx context.Context, since time.Time) (int64, error)
}

// ---- 设置 ----

type SettingModel interface {
	GetJSON(ctx context.Context, key string) ([]byte, bool, error)
	SetJSON(ctx context.Context, key string, value []byte) error
}

// ---- 订阅 ----

type SubscriptionHost struct {
	ID             int64
	Host           string
	EntryGroupID   string
	EntryGroupName string
	TLSMode        string
	TLSStatus      string
	CertNotAfter   *time.Time
	CreatedAt      time.Time
}

type SubscriptionFormat struct {
	Key                 string
	Label               string
	Enabled             bool
	SortOrder           int
	Source              string
	CapabilitySupported bool
}

type SubscriptionModel interface {
	ListHosts(ctx context.Context, page, pageSize int) ([]SubscriptionHost, int64, error)
	GetHost(ctx context.Context, id int64) (*SubscriptionHost, error)
	GetHostByName(ctx context.Context, host string) (*SubscriptionHost, error)
	CreateHost(ctx context.Context, h SubscriptionHost) (*SubscriptionHost, error)
	UpdateHost(ctx context.Context, id int64, entryGroupID, tlsMode *string) (*SubscriptionHost, error)
	DeleteHost(ctx context.Context, id int64) error
	ListFormats(ctx context.Context) ([]SubscriptionFormat, error)
	SetFormatEnabled(ctx context.Context, key string, enabled bool, sortOrder int) error
}

// ---- 通知 ----

type NotificationChannel struct {
	ID              int64
	Name            string
	Type            string
	TargetEncrypted []byte
	TargetMasked    string
	MinLevel        string
	Enabled         bool
	LastSentAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type NotificationPatch struct {
	Name            *string
	TargetEncrypted []byte
	TargetMasked    *string
	MinLevel        *string
	Enabled         *bool
}

type NotificationModel interface {
	List(ctx context.Context) ([]NotificationChannel, error)
	Get(ctx context.Context, id int64) (*NotificationChannel, error)
	Create(ctx context.Context, c NotificationChannel) (*NotificationChannel, error)
	Update(ctx context.Context, id int64, p NotificationPatch) (*NotificationChannel, error)
	Delete(ctx context.Context, id int64) error
	MarkSent(ctx context.Context, id int64, at time.Time) error
}

// ---- Geo ----

type GeoFile struct {
	ID              int64
	Kind            string
	Name            string
	Version         string
	SizeBytes       int64
	Checksum        string
	CategoriesCount int
	Status          string
	Path            string
	UpdatedAt       time.Time
}

type GeoCategory struct {
	Category     string
	EntriesCount int
}

type GeoModel interface {
	List(ctx context.Context, kind string, page, pageSize int) ([]GeoFile, int64, error)
	Get(ctx context.Context, id int64) (*GeoFile, error)
	Upsert(ctx context.Context, f GeoFile) (*GeoFile, error)
	ReplaceCategories(ctx context.Context, fileID int64, cats []GeoCategory) error
	Categories(ctx context.Context, fileID int64, page, pageSize int) ([]GeoCategory, int64, error)
}

// ---- 审计 ----

type AuditLog struct {
	ID         int64
	Actor      string
	Action     string
	ObjectType string
	ObjectID   string
	Result     string
	RequestID  string
	IP         string
	CreatedAt  time.Time
}

type AuditFilter struct {
	Page       int
	PageSize   int
	From       *time.Time
	To         *time.Time
	Actor      string
	Action     string
	ObjectType string
	ObjectID   string
	RequestID  string
}

type AuditLogModel interface {
	Insert(ctx context.Context, l AuditLog) error
	List(ctx context.Context, f AuditFilter) ([]AuditLog, int64, error)
	Get(ctx context.Context, id int64) (*AuditLog, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

// ---- 设置下发任务（探测规格） ----

type SettingsApplyTask struct {
	ID        string
	Kind      string
	State     string
	CreatedBy string
	RequestID string
	CreatedAt time.Time
	UpdatedAt time.Time
	Nodes     []SettingsApplyNode
}

type SettingsApplyNode struct {
	TaskID    string
	NodeID    string
	NodeName  string
	State     string
	Version   int64
	ElapsedMs int64
	ErrorCode string
	UpdatedAt time.Time
}

type SettingsApplyModel interface {
	CreateTask(ctx context.Context, t *SettingsApplyTask) error
	GetTask(ctx context.Context, id string) (*SettingsApplyTask, error)
	LatestTask(ctx context.Context, kind string) (*SettingsApplyTask, error)
	UpsertNode(ctx context.Context, n *SettingsApplyNode) error
	ListNodes(ctx context.Context, taskID string) ([]SettingsApplyNode, error)
}

// ---- 升级 ----
type UpgradeTask struct {
	ID            string
	Scope         string
	TargetVersion string
	FromVersion   string
	BatchID       string
	Status        string
	ErrorCode     string
	ErrorMessage  string
	CreatedBy     string
	RequestID     string
	CreatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time
	Nodes         []UpgradeTaskNode
}

type UpgradeTaskNode struct {
	TaskID       string
	NodeID       string
	NodeName     string
	FromVersion  string
	ToVersion    string
	State        string
	ErrorCode    string
	ErrorMessage string
	UpdatedAt    time.Time
}

type UpgradeLock struct {
	Holder      string
	Scope       string
	AcquiredAt  time.Time
	HeartbeatAt time.Time
	ExpiresAt   time.Time
}

type UpgradeTarget struct {
	NodeID        string
	NodeName      string
	TargetVersion string
	BatchID       string
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

type UpgradeTaskFilter struct {
	Page     int
	PageSize int
	Scope    string
	Status   string
}

type UpgradeModel interface {
	CreateTask(ctx context.Context, t *UpgradeTask) error
	GetTask(ctx context.Context, id string) (*UpgradeTask, error)
	ListTasks(ctx context.Context, f UpgradeTaskFilter) ([]UpgradeTask, int64, error)
	UpdateTaskStatus(ctx context.Context, id, status, errorCode, errorMessage string, startedAt, finishedAt *time.Time) error
	UpsertTaskNode(ctx context.Context, n *UpgradeTaskNode) error
	ListTaskNodes(ctx context.Context, taskID string) ([]UpgradeTaskNode, error)
	ActiveTask(ctx context.Context) (*UpgradeTask, error)

	AcquireLock(ctx context.Context, holder, scope string, now time.Time, ttl time.Duration) (bool, error)
	GetLock(ctx context.Context) (*UpgradeLock, error)
	RenewLock(ctx context.Context, holder string, now time.Time, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, holder string) error

	UpsertTarget(ctx context.Context, t UpgradeTarget) error
	ListTargets(ctx context.Context) ([]UpgradeTarget, error)
	DeleteTarget(ctx context.Context, nodeID string) error
	DeleteTargetsByBatch(ctx context.Context, batchID string) error
}

var (
	_ NodeModel          = (*PostgresNodeModel)(nil)
	_ AlertModel         = (*PostgresAlertModel)(nil)
	_ TrafficModel       = (*PostgresTrafficModel)(nil)
	_ ClientModel        = (*PostgresClientModel)(nil)
	_ SettingModel       = (*PostgresSettingModel)(nil)
	_ SubscriptionModel  = (*PostgresSubscriptionModel)(nil)
	_ NotificationModel  = (*PostgresNotificationModel)(nil)
	_ GeoModel           = (*PostgresGeoModel)(nil)
	_ AuditLogModel      = (*PostgresAuditLogModel)(nil)
	_ UpgradeModel       = (*PostgresUpgradeModel)(nil)
	_ SettingsApplyModel = (*PostgresSettingsApplyModel)(nil)
)

// timeArg 把 nil *time.Time 转为无类型 nil：go-zero sqlx 的语句日志无法格式化
// 带类型的 nil 指针（会 panic），必须在传参前收敛。
func timeArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
