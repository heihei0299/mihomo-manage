Status: completed

# 01 — 基础设施 + 安装验证

## Parent

`.scratch/acceptance-tests/PRD.md`

## What to build

验收测试套件的基础设施 + 第一个验收测试（安装）。

基础设施包括：
- build tag `//go:build acceptance` 隔离验收测试
- 测试辅助函数：编译当前源码为临时二进制、以子进程运行 `mihomo-manager <command>` 并捕获 stdout/stderr/退出码
- 前置检查：`sudo -n true`、`systemctl` 可用性、网络连通性
- 清理函数：卸载已安装的 mihomo 恢复环境

安装测试（`TestAcceptanceInstall`）：
1. 确保 mihomo 未安装
2. 运行 `sudo mihomo-manager install`
3. 断言输出包含 5 个阶段（fetch → deploy → bootstrap → register → start）及对应标记（`✓` / 空格）
4. 断言退出码为 0
5. 验证安装后文件：`/opt/mihomo/bin/mihomo`、`/opt/mihomo/etc/config-template.yaml`（含 `{{subscription}}`）、`/opt/mihomo/etc/config.yaml`（含 `port: 7890`）
6. 验证 `systemctl is-active mihomo` → `active`

## Acceptance criteria

- [ ] `go test -tags=acceptance ./acceptance/ -run TestAcceptanceInstall -count=1` 通过
- [ ] 安装失败时（如网络不可达）输出 `✗ [fetch]`，退出码非 0

## Blocked by

None — can start immediately
