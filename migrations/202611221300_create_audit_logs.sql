-- +goose Up
-- +goose StatementBegin
CREATE TABLE audit_logs (
    id         bigserial    PRIMARY KEY,
    actor      varchar(128) NOT NULL DEFAULT '',
    action     varchar(64)  NOT NULL,
    object     varchar(256) NOT NULL DEFAULT '',
    request_id varchar(128) NOT NULL DEFAULT '',
    result     varchar(32)  NOT NULL,
    created_at timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX ix_audit_logs_created ON audit_logs (created_at DESC);
CREATE INDEX ix_audit_logs_object ON audit_logs (object);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE audit_logs;
-- +goose StatementEnd
