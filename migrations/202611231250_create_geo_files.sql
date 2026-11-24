-- +goose Up
-- +goose StatementBegin
CREATE TABLE geo_files (
    id               bigserial    PRIMARY KEY,
    kind             varchar(16)  NOT NULL,
    name             varchar(128) NOT NULL,
    version          varchar(64)  NOT NULL DEFAULT '',
    size_bytes       bigint       NOT NULL DEFAULT 0,
    checksum         varchar(64)  NOT NULL DEFAULT '',
    categories_count integer      NOT NULL DEFAULT 0,
    status           varchar(16)  NOT NULL DEFAULT 'ready',
    path             text         NOT NULL,
    updated_at       timestamptz  NOT NULL DEFAULT now(),
    UNIQUE (kind, name, version)
);

CREATE TABLE geo_file_categories (
    file_id       bigint       NOT NULL REFERENCES geo_files(id) ON DELETE CASCADE,
    category      varchar(128) NOT NULL,
    entries_count integer      NOT NULL DEFAULT 0,
    PRIMARY KEY (file_id, category)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE geo_file_categories;
DROP TABLE geo_files;
-- +goose StatementEnd
