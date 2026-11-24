# relayx-upgrader 部署与操作

> 对应 STO-004 BE-8 / OPS-1。relayx-upgrader 是 Panel 自升级的外挂进程：Panel 无安装目录写权限，
> 软链切换与服务重启必须由独立 systemd 服务执行（design.md §7.3）。

## 1. 目录与权限

```text
/opt/relayx/
├── current -> releases/1.3.0        # 原子切换的软链
└── releases/
    ├── 1.3.0/relayx-panel relayx-core-control relayx-upgrader
    └── 1.4.0/...
/run/relayx/upgrader.sock            # 0660 root:relayx
/etc/relayx/upgrader.token           # 0600 root:root（Panel 与 upgrader 共享）
/var/lib/relayx-upgrader/state.json  # 0600 root:relayx
```

```bash
install -d -o root -g relayx -m 2770 /opt/relayx/releases /run/relayx /var/lib/relayx-upgrader
install -d -o root -g root -m 0750 /etc/relayx
openssl rand -hex 32 > /etc/relayx/upgrader.token
chmod 0600 /etc/relayx/upgrader.token
```

## 2. 安装 systemd 单元

```bash
install -m 0644 packaging/systemd/relayx-upgrader.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now relayx-upgrader.service
systemctl status relayx-upgrader.service
```

单元使用 `ProtectSystem=strict` + `ReadWritePaths=/opt/relayx /var/lib/relayx-upgrader /run/relayx`，
只能重启 `relayx-panel.service` 与 `relayx-core-control.service`（design.md §7.3 / AC112）。

## 3. Panel 配置

```yaml
Upgrader:
  Enabled: true
  SocketPath: /run/relayx/upgrader.sock
  TokenFile: /etc/relayx/upgrader.token
  Timeout: 30m
Deployment:
  Mode: binary      # compose 时 Panel 只返回 docker compose pull && up -d 提示
Update:
  SourceURL: https://updates.example.com/panel/manifest.json
  Channel: stable
  PublicKeys: ["<ed25519-hex>"]
```

## 4. 排障

```bash
# 状态（无需重启 Panel）
curl --unix-socket /run/relayx/upgrader.sock \
  -H "X-Relayx-Upgrader-Token: $(cat /etc/relayx/upgrader.token)" \
  http://localhost/status

journalctl -u relayx-upgrader -f
cat /var/lib/relayx-upgrader/state.json
```

- `UPSTREAM_SIGNATURE_INVALID`：manifest/资产签名或 SHA-256 不匹配；检查 `Update.PublicKeys` 与发布流水线。
- `KERNEL_UNHEALTHY`：重启后 `/readyz` 未在 60s 内返回 200；upgrader 已自动切回上一版本。
- `UPGRADE_ROLLED_BACK`：升级失败并已回滚；查看 state.json 的 `error` 字段。

## 5. 发布顺序

1. 先发布 `relayx-upgrader`（兼容旧 Panel），再发布 Panel（Panel 拉起后即可发起自升级）。
2. 更新 `manifest.json` 与 `.sig` 后，Panel 每 6 小时检查一次；也可在「版本与升级」手动检查。
3. Compose 部署不自动重建容器：Panel 返回 `docker compose pull && docker compose up -d` 提示。
