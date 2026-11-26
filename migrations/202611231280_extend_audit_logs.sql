-- +goose Up
-- +goose StatementBegin
ALTER TABLE audit_logs ADD COLUMN object_type varchar(32)  NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN object_id   varchar(128) NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN ip          varchar(64)  NOT NULL DEFAULT '';
CREATE INDEX ix_audit_logs_type_object ON audit_logs (object_type, object_id, created_at DESC);
CREATE INDEX ix_audit_logs_request ON audit_logs (request_id);

-- 既有记录只有 object 文本，回填到 object_id（design.md §6.2）。
UPDATE audit_logs SET object_id = object WHERE object_id = '' AND object <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX ix_audit_logs_request;
DROP INDEX ix_audit_logs_type_object;
ALTER TABLE audit_logs DROP COLUMN ip;
ALTER TABLE audit_logs DROP COLUMN object_id;
ALTER TABLE audit_logs DROP COLUMN object_type;
-- +goose StatementEnd
