# STO-002 下发编排部署与配置说明

> 面向运维 / 部署负责人。配置样例见各仓 `etc/*.yaml`（带注释的生产必配项）。

## 1. 组件与端口

| 组件 | 监听 | 说明 |
|---|---|---|
| Panel API | `Host:Port`（默认 8888） | 对外 HTTP（本期不新增接口） |
| Panel RegistrationService | `Registration.ListenOn`（默认 127.0.0.1:9101） | 节点注册 gRPC（design §3.10） |
| core-control | `127.0.0.1:9000` | 随 Panel 部署，仅回环 / 内网 |
| Agent | `0.0.0.0:9100` | Panel → Agent gRPC（mTLS） |
| core-node | `/run/relayx/core-node.sock` | 仅 Unix socket，无 TCP |

## 2. 安全前置（生产必配）

### 2.1 Panel Ed25519 公钥
- Panel 持有 Ed25519 签名私钥；core-control / Agent 配置 `PanelPublicKey`（base64 公钥）。
- 未配置时仅校验时间戳 / nonce（开发模式），**签名不校验**。

### 2.2 mTLS
- core-control / Agent 服务端：`TLS{CertFile,KeyFile,CAFile}`（`RequireAndVerifyClientCert`，TLS 1.3）。
- Panel 客户端：`CoreControl` / `Agent` 的 `CertFile/KeyFile/CAFile/ServerName`（`Insecure: false`）。
- RegistrationService：`Registration.CertFile/KeyFile/CAFile`（`RequestClientCert`：Register 免客户端证书，Renew 取 CN）。

### 2.3 Panel CA 与主密钥
- `Registration.CACertFile` / `CAKeyFile`：Panel CA 证书 / 私钥，用于签发节点证书。
- `RELAYX_MASTER_KEY`（环境变量，base64 32 字节）：AES-256-GCM 主密钥，用于配置版本 / 快照 / 材料加密。未设置时 `dispatch.Service` 不构造。

### 2.4 core-node 本地权限
- `AllowGID`：允许访问 Unix socket 的 `relayx` 组 GID；`0` 表示不校验（生产必配）。
- 内置 Xray 统计 API：`XrayAPIAddr`（默认 `127.0.0.1:10085`），供 perception 采集。

## 3. 节点注册时序

1. 运维在 Panel 侧创建一次性 Token（绑定 node_type，24h、单次）。
2. Agent 启动：无证书 → `RegistrationService.Register`（server-auth TLS + Token + CSR）→ 落盘证书/私钥 0600。
3. 之后 Panel ↔ Agent 走 mTLS；证书到期前 7 天经 `Renew` 续期。
4. 吊销：Panel 标记 revoked；Agent 重连被拒 `AUTH_CERT_REVOKED`。

Agent 配置项：`PanelRegistrationAddr` / `PanelCAFile` / `RegistrationToken` / `CertFile` / `KeyFile`。

## 4. 启动顺序

1. Panel（含 core-control sidecar）+ PostgreSQL / Redis。
2. core-node（`relayx-core-node.service`）。
3. Agent（`relayx-agent.service`，依赖 core-node）。

## 5. 开发 / 联调凭据一键生成

```bash
cd projects/panel
go run ./cmd/devcerts -out ../../dev/deploy
```

产出（`dev/deploy/`，权限 0600）：

| 文件 | 对应配置 |
|---|---|
| `panel-ca.crt` / `panel-ca.key` | `Registration.CACertFile` / `CAKeyFile`；各服务 `CAFile` |
| `registration.crt` / `registration.key` | `Registration.CertFile` / `KeyFile` |
| `panel-client.crt` / `panel-client.key` | Panel 作为客户端时的 `CertFile` / `KeyFile` |
| `panel-ed25519.pub` | core-control / Agent 的 `PanelPublicKey`（base64） |
| `panel-ed25519.key` | Panel 签名私钥（由 Panel 持有） |
| `master-key.txt` | 环境变量 `RELAYX_MASTER_KEY` |

> 仅用于开发 / 联调；生产请使用正式 CA 与密钥托管流程，`dev/deploy/` 不入库。

## 6. 已知限制

- 真实 Xray v26.11.9 集成（`xray -test` / 故障注入 / 500 端口压测）需在 Linux 环境执行。
- 编排（dispatch）在 `ServiceContext` 暴露，供内部调用；本期不新增对外 HTTP 触发接口。
