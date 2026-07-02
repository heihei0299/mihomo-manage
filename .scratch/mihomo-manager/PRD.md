Status: completed

# mihomo-manager: 本地 mihomo 代理实例管理器

## Problem Statement

mihomo（Clash Meta）是一个高性能代理核心，但缺乏一个用户友好的本地管理工具。用户需要手动下载二进制、编写配置文件、管理 systemd 服务、手动检查更新——所有这些操作都没有统一的入口。对于非深度用户，这构成了使用门槛；对于日常用户，反复的手动操作造成了效率损耗。

需要一个工具，将 mihomo 实例的完整生命周期（安装、配置、运行控制、升级、卸载）整合到一个统一的终端界面中。

## Solution

**mihomo-manager** 是一个运行在本机的管理工具，TUI 为主、CLI 为辅。它将一个 mihomo 实例从安装到日常使用到最终卸载的所有操作纳入一个统一的工作流：

- **安装**：自动下载 mihomo 二进制、部署到系统目录、注册 systemd/launchd 服务、生成初始配置
- **配置**：通过模板引擎将订阅配置（远程 URL 或本地粘贴）与用户自定义分流规则合并，生成最终配置
- **运行控制**：启动、停止、重启、重载配置，查看实例运行状态
- **升级**：从 GitHub Releases 获取版本列表，用户选择目标版本，自动替换并回滚
- **卸载**：清理二进制、配置、服务注册，用户可选择保留备份

## User Stories

1. As a user, I want to install mihomo with a single command, so that I don't need to manually download and set up the binary
2. As a user, I want to see the installation progress (download, deploy, register, start), so that I know what's happening
3. As a user, I want to receive clear error messages if installation fails at any step, so that I can diagnose the problem
4. As a user, I want to add a remote subscription URL, so that my proxy nodes are automatically pulled from my provider
5. As a user, I want to paste local subscription content directly, so that I can use configs that aren't hosted remotely
6. As a user, I want to edit the config template file, so that I can customize the structure of the generated mihomo config
7. As a user, I want to edit my custom routing rules, so that I can define which traffic goes through which proxy
8. As a user, I want to preview the final generated config before it's applied, so that I can verify the result of template rendering
9. As a user, I want to trigger a manual subscription refresh, so that I can get the latest proxy nodes immediately
10. As a user, I want to configure scheduled subscription auto-refresh, so that my proxy nodes stay up to date without manual intervention
11. As a user, I want the current config to be automatically backed up before each subscription update, so that I can roll back if the new config breaks something
12. As a user, I want to start the mihomo instance, so that I can use the proxy service
13. As a user, I want to stop the mihomo instance, so that I can temporarily disable the proxy
14. As a user, I want to restart the mihomo instance, so that I can apply changes that require a full restart
15. As a user, I want to reload the mihomo config without restarting the process, so that config changes take effect without downtime
16. As a user, I want to view the current instance state (running/stopped/failed), so that I know the status of my proxy at a glance
17. As a user, I want to check for available mihomo versions, so that I know if a newer release exists
18. As a user, I want to see the last 5 available versions and select which one to upgrade to, so that I can choose to stay on a known-good version
19. As a user, I want the upgrade process to automatically back up the current binary, so that I can roll back if needed
20. As a user, I want automatic rollback on upgrade failure (new version fails to start), so that my proxy service is restored without manual recovery
21. As a user, I want to see upgrade progress (download, stop, replace, start), so that I know what's happening during the process
22. As a user, I want to uninstall mihomo, so that I can cleanly remove it from my system
23. As a user, I want to choose whether to keep a backup of configs and binary when uninstalling, so that I can reinstall later without losing my setup
24. As a user, I want to perform all operations through a TUI dashboard, so that I have a unified visual interface for managing my proxy
25. As a user, I want the TUI to show the current instance state prominently, so that I can see the status at a glance
26. As a user, I want to perform all operations through CLI commands, so that I can script and automate common tasks
27. As a user, I want CLI commands to return meaningful exit codes, so that I can use them in scripts
28. As a user, I want CLI commands to support a silent/quiet mode, so that I can suppress output in automated contexts
29. As a user, I want to see log output from the mihomo instance through the manager, so that I can troubleshoot proxy issues
30. As a user, I want the initial config template to include sensible defaults, so that I can get started without writing a template from scratch

## Implementation Decisions

- **单一 seam 架构**：所有业务逻辑封装在 `Manager` 模块中，TUI 和 CLI 作为纯胶水层。测试仅针对 `Manager`。
- **模板引擎**：使用简单字符串替换（`{{subscription}}`、`{{routing_rules}}`），理由见 ADR-0001。
- **文件系统布局**：按 CONTEXT.md 中定义的 filesystem-layout 执行。manager 位于 `/opt/mihomo-manager/`，instance 位于 `/opt/mihomo/`。
- **实例状态机**：状态按 CONTEXT.md 中定义的 instance-state 实现（stopped → running → upgrading → failed），操作按 control-operation 实现。
- **服务管理**：Linux 使用 systemd，macOS 使用 launchd。
- **版本来源**：mihomo 版本列表从 GitHub Releases 获取，展示最近 5 个版本，默认选中 latest。
- **备份策略**：订阅更新前自动备份 `config.yaml` 为 `config.yaml.bak.<timestamp>`；升级前备份二进制文件；卸载时用户选择是否保留备份。
- **CLI 接口**：`mihomo-manager <command> [flags]`，每个 control-operation 映射为一个子命令。

### CLI 接口

```
mihomo-manager install [--version <tag>]
mihomo-manager uninstall [--keep-backup]
mihomo-manager start
mihomo-manager stop
mihomo-manager restart
mihomo-manager reload
mihomo-manager status
mihomo-manager upgrade [--version <tag>]
mihomo-manager subscription add [--url <url> | --file <path>]
mihomo-manager subscription update [--force]
mihomo-manager template edit          # opens $EDITOR
mihomo-manager rules edit             # opens $EDITOR
mihomo-manager config preview
mihomo-manager versions               # list available versions
```

## Testing Decisions

- **测试什么**：只测试 `Manager` 模块的外部行为，不测试实现细节。每个 lifecycle（install/upgrade/uninstall）作为一个整体场景测试输入输出和状态变化。
- **什么构成好的测试**：
  - 给定某种系统状态（mihomo 未安装 / 已安装 / 正在运行），执行操作后验证最终系统状态
  - 测试状态机转换的合法性（非法操作应返回错误，不改变状态）
  - 通过 mock 文件系统和进程管理器来测试 lifecycle 逻辑，不实际执行二进制文件
  - 测试 config-pipeline 的输入输出：给定原始订阅数据 + 模板 + 路由规则，验证最终 config.yaml 内容
- **测试模块**：单一 seam — `Manager` 模块。TUI 和 CLI 层不做集成测试（纯逻辑转发，风险极低）。
- **无先例**：greenfield 项目，这是首次建立测试惯例。
- **测试文件组织**：`Manager` 模块的同级 `_test.go`（或对应语言惯例的测试文件）。

## Out of Scope

- **远程管理**：不支持管理其他机器上的 mihomo 实例。本工具限定为单机本地工具。
- **多实例**：不支持在同一台机器上管理多个 mihomo-instance。
- **TUI 集成测试**：TUI 的渲染和交互逻辑不做自动化测试（手动验收）。
- **mihomo 日志分析**：不做日志解析、统计、告警。仅透传 mihomo 的标准输出/错误。
- **图形界面**：无 GUI（GTK/Qt/Web）计划。仅限 TUI + CLI。
- **代理协议实现**：不实现任何代理协议。代理功能完全委托给 mihomo 核心。

## Further Notes

- 本 PRD 覆盖 mihomo-manager 的完整初始版本。实现可按垂直切片拆分（见 Issue Tracker 中分解的子 issue）。
- 实现语言未在此 PRD 中指定，由实现阶段根据生态和技术考量决定。TUI 库的选择可能影响语言选择（如 Go + Bubble Tea、Rust + Ratatui 等）。
- 配置备份文件位置：与 `config.yaml` 同目录，命名 `config.yaml.bak.<unix-timestamp>`。
