-- +goose Up
-- +goose StatementBegin
CREATE TABLE node_events (
    id          bigserial    PRIMARY KEY,
    node_id     varchar(128) NOT NULL,
    type        varchar(32)  NOT NULL,
    level       varchar(16)  NOT NULL,
    message     text         NOT NULL,
    metadata    jsonb        NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz  NOT NULL
);
CREATE INDEX ix_node_events_node_time ON node_events (node_id, occurred_at DESC);
CREATE INDEX ix_node_events_time ON node_events (occurred_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE node_events;
-- +goose StatementEnd
