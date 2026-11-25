-- +goose Up
-- +goose StatementBegin
-- dependency: STO-002 —— 本迁移只补齐 STO-004 读模型所需的最小依赖表 / 列。
-- STO-002 / OPS-005 合入后以其迁移为准；本文件使用 IF NOT EXISTS 保证重复执行安全。
CREATE TABLE IF NOT EXISTS entry_groups (
    id     varchar(128) PRIMARY KEY,
    name   varchar(128) NOT NULL,
    status varchar(16)  NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS tunnel_groups (
    id     varchar(128) PRIMARY KEY,
    name   varchar(128) NOT NULL,
    status varchar(16)  NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS port_groups (
    id              varchar(128) PRIMARY KEY,
    name            varchar(128) NOT NULL,
    status          varchar(32)  NOT NULL DEFAULT 'active',
    entry_group_id  varchar(128) NOT NULL DEFAULT '',
    tunnel_group_id varchar(128) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS ports (
    id                  varchar(128) PRIMARY KEY,
    port_group_id       varchar(128) NOT NULL,
    port                integer      NOT NULL,
    protocol            varchar(16)  NOT NULL DEFAULT 'tcp',
    forward_mode        varchar(32)  NOT NULL DEFAULT 'direct',
    subscription_hidden boolean      NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS tunnel_ports (
    id              varchar(128) PRIMARY KEY,
    tunnel_group_id varchar(128) NOT NULL,
    port_group_id   varchar(128) NOT NULL,
    port_id         varchar(128) NOT NULL,
    port            integer      NOT NULL,
    protocol        varchar(16)  NOT NULL DEFAULT 'tcp',
    status          varchar(16)  NOT NULL DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS outbounds (
    id               varchar(128) PRIMARY KEY,
    port_group_id    varchar(128) NOT NULL,
    type             varchar(32)  NOT NULL,
    config_encrypted bytea        NULL
);

CREATE TABLE IF NOT EXISTS nodes (
    id                   varchar(128) PRIMARY KEY,
    name                 varchar(128) NOT NULL,
    type                 varchar(16)  NOT NULL DEFAULT 'entry',
    group_id             varchar(128) NOT NULL DEFAULT '',
    country              varchar(64)  NOT NULL DEFAULT '',
    province             varchar(64)  NOT NULL DEFAULT '',
    region               varchar(128) NOT NULL DEFAULT '',
    status               varchar(16)  NOT NULL DEFAULT 'offline',
    ip                   varchar(64)  NOT NULL DEFAULT '',
    agent_version        varchar(64)  NOT NULL DEFAULT '',
    xray_version         varchar(64)  NOT NULL DEFAULT '',
    last_seen_at         timestamptz  NULL,
    cert_fingerprint     varchar(128) NOT NULL DEFAULT '',
    cert_not_after       timestamptz  NULL,
    cpu_percent          double precision NOT NULL DEFAULT 0,
    mem_percent          double precision NOT NULL DEFAULT 0,
    disk_percent         double precision NOT NULL DEFAULT 0,
    load1                double precision NOT NULL DEFAULT 0,
    tcp_connections      integer      NOT NULL DEFAULT 0,
    net_in_bps           double precision NOT NULL DEFAULT 0,
    net_out_bps          double precision NOT NULL DEFAULT 0,
    xray_state           varchar(16)  NOT NULL DEFAULT 'unknown',
    xray_restarts        integer      NOT NULL DEFAULT 0,
    xray_pid             integer      NOT NULL DEFAULT 0,
    xray_cpu_percent     double precision NOT NULL DEFAULT 0,
    xray_mem_bytes       bigint       NOT NULL DEFAULT 0,
    xray_inbounds        integer      NOT NULL DEFAULT 0,
    xray_outbounds       integer      NOT NULL DEFAULT 0,
    nics                 jsonb        NOT NULL DEFAULT '[]'::jsonb,
    updated_at           timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_nodes_status ON nodes (status, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS ix_nodes_group ON nodes (type, group_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS nodes;
DROP TABLE IF EXISTS outbounds;
DROP TABLE IF EXISTS tunnel_ports;
DROP TABLE IF EXISTS ports;
DROP TABLE IF EXISTS port_groups;
DROP TABLE IF EXISTS tunnel_groups;
DROP TABLE IF EXISTS entry_groups;
-- +goose StatementEnd
