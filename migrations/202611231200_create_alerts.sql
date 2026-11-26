-- +goose Up
-- +goose StatementBegin
CREATE TABLE alerts (
    id           bigserial    PRIMARY KEY,
    type         varchar(64)  NOT NULL,
    level        varchar(16)  NOT NULL,
    object_type  varchar(32)  NOT NULL,
    object_id    varchar(128) NOT NULL DEFAULT '',
    message      text         NOT NULL,
    status       varchar(16)  NOT NULL,
    first_seen_at timestamptz NOT NULL,
    last_seen_at  timestamptz NOT NULL,
    resolved_at   timestamptz NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_alerts_status_level ON alerts (status, level, last_seen_at DESC);
CREATE INDEX ix_alerts_object ON alerts (object_type, object_id);

CREATE TABLE notification_channels (
    id              bigserial    PRIMARY KEY,
    name            varchar(128) NOT NULL,
    type            varchar(32)  NOT NULL,
    target_encrypted bytea       NOT NULL,
    target_masked   varchar(256) NOT NULL DEFAULT '',
    min_level       varchar(16)  NOT NULL,
    enabled         boolean      NOT NULL DEFAULT true,
    last_sent_at    timestamptz  NULL,
    created_at      timestamptz  NOT NULL DEFAULT now(),
    updated_at      timestamptz  NOT NULL DEFAULT now()
);

CREATE TABLE alert_notifications (
    id         bigserial    PRIMARY KEY,
    alert_id   bigint       NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    channel_id bigint       NOT NULL,
    status     varchar(16)  NOT NULL,
    error_code varchar(64)  NOT NULL DEFAULT '',
    sent_at    timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX ix_alert_notifications_alert ON alert_notifications (alert_id, sent_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE alert_notifications;
DROP TABLE notification_channels;
DROP TABLE alerts;
-- +goose StatementEnd
