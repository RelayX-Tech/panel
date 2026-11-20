-- +goose Up
-- +goose StatementBegin
CREATE TABLE auth_users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account       varchar(64)  NOT NULL,
    password_hash text         NOT NULL,
    display_name  varchar(128) NOT NULL DEFAULT '',
    disabled_at   timestamptz  NULL,
    created_at    timestamptz  NOT NULL DEFAULT now(),
    updated_at    timestamptz  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX uq_auth_users_account_lower ON auth_users (lower(account));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE auth_users;
-- +goose StatementEnd
