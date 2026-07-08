# 领域术语

## mihomo-instance

安装到本机的一套完整 mihomo 软件，包含二进制文件、配置文件、系统服务（systemd/launchd）以及运行时数据。一个实例即可独立提供代理服务。

## manager

运行在本机的管理工具（即本项目），负责 mihomo-instance 的生命周期管理。

## installation-lifecycle

mihomo-instance 从无到有再到正常运行所经历的阶段序列，依次为：

1. **fetch** — 从上游获取 mihomo 二进制文件
2. **deploy** — 将二进制放置到文件系统目标位置
3. **bootstrap** — 创建配置目录及初始配置文件
4. **register** — 写入 service unit 文件并通知系统守护进程（systemd: `daemon-reload`；launchd: `launchctl load`）。**始终执行**。
5. **enable-auto-start** — 条件执行，仅当用户选择开机自启时启用（Linux: `systemctl enable`；Darwin: plist 含 `RunAtLoad`/`KeepAlive`）
6. **start** — 首次启动服务

## routing-rule

用户自定义的分流规则，以原始 mihomo 兼容语法编写（如 `DOMAIN-SUFFIX,google.com,Proxy`）。模板在渲染时直接拼接到最终配置文件中。

## subscription-source

订阅配置的数据来源。支持两种：

- **remote** — 从 URL 订阅链接拉取
- **local** — 用户手动粘贴的配置内容

## subscription-update

刷新订阅配置的方式：

- **manual** — 用户通过 TUI/CLI 手动触发刷新
- **scheduled** — 按设定周期自动刷新（如每天一次），刷新后重新执行 config-pipeline 并 reload 实例

每次更新前自动备份当前配置文件。备份文件命名格式 `config.yaml.bak.<timestamp>`。

## config-pipeline

从原始订阅数据到 mihomo 最终配置文件的转换流水线，封装为一个独立的模块（`ConfigPipeline` 接口）：

1. 从 subscription-source 获取原始数据（remote URL → HTTP 下载，local → 直接读取）
2. 经过模板渲染（`renderConfig` 简单字符串替换），合并 routing-rule
3. 备份当前配置文件
4. 输出最终 mihomo 配置文件
5. 调用 `mihomo -t` 验证配置合法性；失败则自动回滚至备份
6. 通过 `OnReload` 回调信号通知重载实例

**模块边界：** 仅覆盖转换管道本身，不含 install 引导阶段的初始配置写入。
**重载信号：** 通过注入回调（`OnReload func(ctx) error`）离开模块，不直接依赖 ServiceManager。
**配置验证：** 通过独立的 `ConfigValidator` seam 注入，production 调 `mihomo -t`，test 可替换为假验证器。

## uninstall-lifecycle

移除 mihomo-instance 的流程：

1. **stop** — 停止当前实例
2. **deregister** — 移除系统服务注册（systemd/launchd）
3. **cleanup** — 删除二进制及配置目录。用户可选择是否保留备份

## upgrade-lifecycle

升级 mihomo-instance 的二进制版本的流程：

1. **check** — 从 GitHub Releases 获取可用版本列表
2. **select** — 用户选择版本（展示最近5个版本，默认选中 latest）
3. **fetch** — 下载所选版本的二进制
4. **stop** — 停止当前实例
5. **replace** — 替换二进制文件（旧版本备份到指定位置以备回滚）
6. **start** — 启动新版本实例
7. **rollback-on-fail** — 如启动失败，自动回滚至备份的旧版本并恢复运行

## config-template

用户可编辑的模板文件，使用**简单字符串替换**（非 Jinja2/Go template 等引擎）。模板中预留替换点位：

- `{{subscription}}` — 订阅数据的占位
- `{{routing_rules}}` — 用户自定义分流规则占位

模板文件存放于实例的配置目录中。

## auto-start

mihomo-instance 是否在系统启动时自动运行。独立于 `instance-state`，通过 `Status.AutoStartEnabled` 字段暴露。

- **Linux:** 由 `systemctl is-enabled` 判定；`systemctl enable/disable` 切换
- **Darwin:** 由 plist 中是否包含 `RunAtLoad` 和 `KeepAlive` 判定；通过重写 plist 并 reload 切换

## instance-state

mihomo-instance 的运行状态：

| 状态 | 说明 |
|------|------|
| `stopped` | 未运行 |
| `running` | 正常运行 |
| `upgrading` | 升级中（替换二进制后重启） |

## filesystem-layout

manager 在文件系统中的目录结构：

```
/opt/mihomo-manager/            ← manager 安装目录
  └── bin/                      ← 自身二进制
  └── state/                    ← 内部运行时数据

/opt/mihomo/                    ← mihomo-instance
  ├── bin/
  │   └── mihomo                ← mihomo 二进制
  ├── etc/
  │   ├── config-template.yaml  ← 用户可编辑的模板
  │   ├── config.yaml           ← 最终生成配置
  │   └── rules.txt             ← 用户自定义分流规则
  └── run/                      ← 运行时文件（pid 等）
```

## interface

manager 的交互方式：

- **TUI** — 主界面，终端交互式界面，覆盖所有功能
- **CLI** — 命令行模式，覆写所有功能但不提供交互式界面，适合脚本调用

## control-operation

用户对 mihomo-instance 施行的控制操作：

- **start** — 启动实例
- **stop** — 停止实例
- **restart** — 重启实例
- **reload** — 重载配置文件（不重启进程）
- **enable-autostart** — 开启开机自启（Linux: `systemctl enable`；Darwin: 写入含 `RunAtLoad` 的 plist 并 reload）
- **disable-autostart** — 关闭开机自启
