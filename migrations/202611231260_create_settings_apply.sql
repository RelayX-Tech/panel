-- +goose Up
-- +goose StatementBegin
CREATE TABLE panel_settings (
    key        varchar(64) PRIMARY KEY,
    value_json jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE settings_apply_tasks (
    id         uuid        PRIMARY KEY,
    kind       varchar(32) NOT NULL,
    state      varchar(16) NOT NULL,
    created_by varchar(128) NOT NULL DEFAULT '',
    request_id varchar(128) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE settings_apply_task_nodes (
    task_id    uuid         NOT NULL REFERENCES settings_apply_tasks(id) ON DELETE CASCADE,
    node_id    varchar(128) NOT NULL,
    state      varchar(16)  NOT NULL,
    version    bigint       NOT NULL DEFAULT 0,
    elapsed_ms bigint       NOT NULL DEFAULT 0,
    error_code varchar(64)  NOT NULL DEFAULT '',
    updated_at timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, node_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE settings_apply_task_nodes;
DROP TABLE settings_apply_tasks;
DROP TABLE panel_settings;
-- +goose StatementEnd
