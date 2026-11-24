-- +goose Up
-- +goose StatementBegin
-- 客户端统计的角色守卫（design.md §3.5 API-12 / AC63）。既有账户默认管理员。
ALTER TABLE auth_users ADD COLUMN role varchar(16) NOT NULL DEFAULT 'admin';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth_users DROP COLUMN role;
-- +goose StatementEnd
