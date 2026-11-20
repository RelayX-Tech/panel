-- +goose Up
-- +goose StatementBegin
CREATE TABLE auth_sessions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_hash        bytea       NOT NULL,
    user_id             uuid        NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    created_at          timestamptz NOT NULL DEFAULT now(),
    last_seen_at        timestamptz NOT NULL DEFAULT now(),
    expires_at          timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at          timestamptz NULL,
    client_ip           inet        NULL,
    user_agent          varchar(512) NULL
);

CREATE UNIQUE INDEX uq_auth_sessions_hash ON auth_sessions (session_hash);
CREATE INDEX idx_auth_sessions_user ON auth_sessions (user_id);
CREATE INDEX idx_auth_sessions_expires ON auth_sessions (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE auth_sessions;
-- +goose StatementEnd
