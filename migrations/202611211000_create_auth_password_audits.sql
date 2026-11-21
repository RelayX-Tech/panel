-- +goose Up
-- +goose StatementBegin
CREATE TABLE auth_password_audits (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL,
    request_id  varchar(64) NOT NULL,
    client_ip   inet        NULL,
    result      varchar(16) NOT NULL DEFAULT 'success',
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_auth_password_audits_user_time ON auth_password_audits (user_id, occurred_at DESC);
CREATE INDEX idx_auth_password_audits_request   ON auth_password_audits (request_id);
CREATE INDEX idx_auth_password_audits_time      ON auth_password_audits (occurred_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE auth_password_audits;
-- +goose StatementEnd
