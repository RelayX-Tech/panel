-- +goose Up
-- +goose StatementBegin
CREATE TABLE upgrade_tasks (
    id             uuid        PRIMARY KEY,
    scope          varchar(16) NOT NULL,
    target_version varchar(64) NOT NULL,
    from_version   varchar(64) NOT NULL DEFAULT '',
    batch_id       uuid        NULL,
    status         varchar(16) NOT NULL,
    error_code     varchar(64) NOT NULL DEFAULT '',
    error_message  text        NOT NULL DEFAULT '',
    created_by     varchar(128) NOT NULL DEFAULT '',
    request_id     varchar(128) NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    started_at     timestamptz NULL,
    finished_at    timestamptz NULL
);
CREATE INDEX ix_upgrade_tasks_created ON upgrade_tasks (created_at DESC);

CREATE TABLE upgrade_task_nodes (
    task_id       uuid         NOT NULL REFERENCES upgrade_tasks(id) ON DELETE CASCADE,
    node_id       varchar(128) NOT NULL,
    from_version  varchar(64)  NOT NULL DEFAULT '',
    to_version    varchar(64)  NOT NULL DEFAULT '',
    state         varchar(16)  NOT NULL,
    error_code    varchar(64)  NOT NULL DEFAULT '',
    error_message text         NOT NULL DEFAULT '',
    updated_at    timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, node_id)
);

CREATE TABLE node_upgrade_targets (
    node_id        varchar(128) PRIMARY KEY,
    target_version varchar(64)  NOT NULL,
    batch_id       uuid         NOT NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    expires_at     timestamptz  NOT NULL
);
CREATE INDEX ix_node_upgrade_targets_expires ON node_upgrade_targets (expires_at);

CREATE TABLE upgrade_lock (
    id           smallint     PRIMARY KEY CHECK (id = 1),
    holder       varchar(128) NOT NULL,
    scope        varchar(64)  NOT NULL,
    acquired_at  timestamptz  NOT NULL,
    heartbeat_at timestamptz  NOT NULL,
    expires_at   timestamptz  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE upgrade_lock;
DROP TABLE node_upgrade_targets;
DROP TABLE upgrade_task_nodes;
DROP TABLE upgrade_tasks;
-- +goose StatementEnd
