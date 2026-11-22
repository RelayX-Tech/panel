-- +goose Up
-- +goose StatementBegin
CREATE TABLE node_registration_tokens (
    id         bigserial    PRIMARY KEY,
    token_hash varchar(64)  NOT NULL UNIQUE,
    node_type  varchar(32)  NOT NULL DEFAULT '',
    expires_at timestamptz  NOT NULL,
    used_at    timestamptz  NULL,
    created_by varchar(128) NOT NULL DEFAULT '',
    created_at timestamptz  NOT NULL DEFAULT now()
);
CREATE INDEX ix_node_registration_tokens_expires ON node_registration_tokens (expires_at);

CREATE TABLE node_certificates (
    id                 bigserial    PRIMARY KEY,
    node_id            varchar(128) NOT NULL,
    serial             varchar(128) NOT NULL DEFAULT '',
    fingerprint_sha256 varchar(64)  NOT NULL,
    not_after          timestamptz  NOT NULL,
    revoked_at         timestamptz  NULL,
    issued_at          timestamptz  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_node_certificates_node ON node_certificates (node_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE node_certificates;
DROP TABLE node_registration_tokens;
-- +goose StatementEnd
