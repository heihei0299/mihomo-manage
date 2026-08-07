# 05 — CLI/TUI 更名与文案

**What to build:** CLI 命令与界面文案与覆写文件命名对齐：`config override edit` 取代 `config template edit`；旧 `template` 与 `config template` 命令输出报错并引导新命令；TUI 配置页显示覆写文件路径与编辑命令；help 文案更新，`template` 字样彻底退出界面。

**Blocked by:** 02（编辑目标为覆写文件）

**Status:** ready-for-agent

- [ ] `config override edit` 打开覆写文件编辑器
- [ ] 旧 `template` / `config template` 命令输出报错并引导新命令（退出码非 0）
- [ ] TUI 配置页显示覆写文件路径与 `override edit` 命令
- [ ] CLI help 无 `template` 字样残留
