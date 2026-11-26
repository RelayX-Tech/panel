-- +goose Up
-- +goose StatementBegin
CREATE TABLE client_stats (
    client_id          varchar(128) PRIMARY KEY,
    port_id            varchar(128) NOT NULL,
    port_group_id      varchar(128) NOT NULL,
    email              varchar(256) NOT NULL DEFAULT '',
    up_bytes           bigint       NOT NULL DEFAULT 0,
    down_bytes         bigint       NOT NULL DEFAULT 0,
    last_online_at     timestamptz  NULL,
    last_sub_fetch_at  timestamptz  NULL,
    updated_at         timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX ix_client_stats_port ON client_stats (port_group_id, port_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE client_stats;
-- +goose StatementEnd
