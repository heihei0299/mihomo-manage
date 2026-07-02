Status: completed

# 03 — 配置管道

## Parent

`.scratch/acceptance-tests/PRD.md`

## What to build

订阅设置、订阅更新、配置预览的验收测试。

### TestAcceptanceSubscriptionSet
- 设置 URL：`mihomo-manager subscription set <URL>` → 退出码 0，`/opt/mihomo-manager/state/subscription-url.txt` 内容匹配
- 设置本地内容：`mihomo-manager subscription set "proxies: [...]"` → `subscription-url.txt` 不可读（os.ErrNotExist），`subscription-data.txt` 包含内容
- 无参数时退出码非 0

### TestAcceptanceSubscriptionUpdate
- 用已知可访问的订阅 URL 执行 update → 退出码 0
- 验证 `/opt/mihomo/etc/config.yaml` 包含 `proxies`
- 验证备份文件 `config.yaml.bak.<timestamp>` 存在
- 验证 `systemctl is-active mihomo` → `active`（reload 不中断服务）
- URL 不可达时退出码非 0

### TestAcceptanceConfigPreview
- 有订阅数据时：输出包含 `port`、`proxies`、`proxy-groups`、`rules`
- 退出码 0

## Acceptance criteria

- [ ] 3 个测试全部通过
- [ ] 每个测试独立可运行

## Blocked by

- #01 — 基础设施 + 安装验证
