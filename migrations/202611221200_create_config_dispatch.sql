-- +goose Up
-- +goose StatementBegin
CREATE TABLE config_operations (
    operation_id uuid         PRIMARY KEY,
    node_id      varchar(128) NOT NULL,
    version      bigint       NOT NULL DEFAULT 0,
    type         varchar(32)  NOT NULL,
    state        varchar(32)  NOT NULL,
    request_id   varchar(128) NOT NULL DEFAULT '',
    error_code   varchar(64)  NOT NULL DEFAULT '',
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX ix_config_operations_node_created ON config_operations (node_id, created_at DESC);

CREATE TABLE config_versions (
    id               bigserial    PRIMARY KEY,
    node_id          varchar(128) NOT NULL,
    port_group_id    varchar(128) NOT NULL DEFAULT '',
    version          bigint       NOT NULL,
    config_hash      varchar(64)  NOT NULL,
    config_encrypted bytea        NOT NULL,
    schema_version   varchar(32)  NOT NULL,
    status           varchar(32)  NOT NULL,
    created_at       timestamptz  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_config_versions_node_version ON config_versions (node_id, version);

CREATE TABLE config_snapshots (
    id                bigserial    PRIMARY KEY,
    node_id           varchar(128) NOT NULL,
    port_group_id     varchar(128) NOT NULL DEFAULT '',
    version           bigint       NOT NULL,
    payload_encrypted bytea        NOT NULL,
    hash              varchar(64)  NOT NULL,
    reason            varchar(64)  NOT NULL DEFAULT '',
    created_at        timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX ix_config_snapshots_node ON config_snapshots (node_id, created_at DESC);

CREATE TABLE tunnel_materials (
    id                 bigserial    PRIMARY KEY,
    tunnel_port_id     varchar(128) NOT NULL,
    kind               varchar(32)  NOT NULL,
    material_encrypted bytea        NOT NULL,
    fingerprint_sha256 varchar(64)  NOT NULL DEFAULT '',
    created_at         timestamptz  NOT NULL DEFAULT now(),
    rotated_at         timestamptz  NULL
);
CREATE INDEX ix_tunnel_materials_port ON tunnel_materials (tunnel_port_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tunnel_materials;
DROP TABLE config_snapshots;
DROP TABLE config_versions;
DROP TABLE config_operations;
-- +goose StatementEnd
