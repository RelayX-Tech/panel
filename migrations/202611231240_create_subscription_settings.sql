-- +goose Up
-- +goose StatementBegin
CREATE TABLE subscription_hosts (
    id             bigserial    PRIMARY KEY,
    host           varchar(253) NOT NULL UNIQUE,
    entry_group_id varchar(128) NOT NULL,
    tls_mode       varchar(16)  NOT NULL DEFAULT 'panel',
    cert_not_after timestamptz  NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now()
);

CREATE TABLE subscription_formats (
    key        varchar(64)  PRIMARY KEY,
    label      varchar(128) NOT NULL,
    enabled    boolean      NOT NULL DEFAULT true,
    sort_order integer      NOT NULL DEFAULT 0,
    updated_at timestamptz  NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE subscription_formats;
DROP TABLE subscription_hosts;
-- +goose StatementEnd
