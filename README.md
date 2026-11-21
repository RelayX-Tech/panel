# panel

RelayX Panel 管理面 API（go-zero）。本仓库当前实现 STO-001 登录与控制台基础建设的后端部分：
`POST /api/v1/auth/login`、`POST /api/v1/auth/logout`、`GET /api/v1/auth/session`。

## 目录

```text
api/panel.api                  # 唯一接口契约（goctl 生成来源）
internal/
  config/                      # 强类型配置
  auth/                        # 密码哈希、会话 token、会话生命周期
  model/                       # auth_users / auth_sessions 数据访问
  migrate/                     # 内嵌迁移器（advisory lock + schema_migrations）
  middleware/                  # Recovery/RequestID/AccessLog/安全头/Origin/Auth/登录限流
  logic/auth/                  # 登录、注销、会话查询
  handler/                     # goctl 生成的薄传输层
  svc/                         # ServiceContext 组合根、初始管理员引导
  response/                    # 统一响应包装与 Cookie
migrations/                    # YYYYMMDDHHMM_description.sql（含 up/down）
docs/openapi/                  # goctl swagger 产物（入库）
tests/integration/             # 真实 PostgreSQL 集成与契约测试
```

## 运行

```bash
# 依赖：Go 1.24+、PostgreSQL
export RELAYX_DB_DSN='postgres://relayx:relayx@127.0.0.1:5432/relayx?sslmode=disable'
export RELAYX_ADMIN_ACCOUNT=admin
export RELAYX_ADMIN_PASSWORD='<至少 12 字符，首次启动后清理>'
go run . -f etc/panel-api.yaml
```

- 开发环境需将 `etc/panel-api.yaml` 的 `Auth.CookieSecure` 置 `false`、`Auth.AllowedOrigins` 配置为前端地址。
- 健康检查：`GET /healthz`、`GET /readyz`（免认证）。
- 生产环境 `Auth.AllowedOrigins` 必须配置实际 HTTPS 域名；密码只经环境变量注入。

## 测试

```bash
# 单测（不需要数据库）
go test $(go list ./... | Select-String -NotMatch 'tests/integration')

# 全部（集成测试需要 PostgreSQL，可用 RELAYX_TEST_DSN 指定）
$env:RELAYX_TEST_DSN='postgres://relayx:relayx@127.0.0.1:55432/relayx_test?sslmode=disable'
go test ./...
```

集成测试会为每个用例创建独立 schema 并自动清理；数据库不可达时跳过。

## 数据模型生成物

`internal/model` 下的 `authusersmodel*.go`、`authsessionsmodel*.go`、`vars.go` 由 **goctl 1.10.2** 生成并入库，禁止手改；重新生成必须零差异。

```bash
# 生成前需对临时库执行迁移（goctl 需要连接真实表结构）
goctl model pg datasource -url="$DSN" -table=auth_users -dir=internal/model -cache

# goctl 1.10.2 的 PostgreSQL 生成器不支持 inet 列；生成 auth_sessions 前，
# 在临时库把 client_ip 转为 text（仅用于生成，生产 schema 不变）：
#   ALTER TABLE auth_sessions ALTER COLUMN client_ip TYPE text USING host(client_ip);
goctl model pg datasource -url="$DSN" -table=auth_sessions -dir=internal/model -cache
```

手写代码与生成物的边界（design.md §5.1.1）：`auth_types.go`（领域类型）、`auth_repository.go`（适配层）、`auth_queries_ext.go`（扩展查询）由人工维护。

## 契约与生成物

```bash
goctl api validate -api api/panel.api
goctl api go -api api/panel.api -dir . -style gozero
goctl api swagger --api api/panel.api --dir docs/openapi --filename panel
# 将生成的 panel.json 重命名为 panel.swagger.json，并删除顶层 "x-date" 字段（保证可重复校验）
go test ./tests/integration -run Swagger
```

契约变更必须先改 `api/panel.api`，同步 OpenAPI 与前端类型，并知会对端（见 `process-docs/STO-001/design.md`）。
