# mihomo-manager

mihomo (Clash Meta) 代理管理工具。管理实例的完整生命周期：安装、配置、升级、卸载。

## 安装

### 从 Release 下载

从 [Releases](https://github.com/heihei0299/mihomo-manage/releases) 下载对应平台的二进制。支持 Linux amd64、Linux arm64、Windows amd64 和 Windows arm64。Linux 需要 systemd，Windows 需要 Windows 10/Server 2016 或更新版本。macOS 不再支持。

```bash
# Linux amd64
sudo install -m 0755 mihomo-manager-linux-amd64 /usr/local/bin/mihomo-manager

# Linux arm64
sudo install -m 0755 mihomo-manager-linux-arm64 /usr/local/bin/mihomo-manager
```

### Windows

使用管理员身份打开 PowerShell，将下载的 `.exe` 保存到固定位置：

```powershell
New-Item -ItemType Directory -Force "$env:ProgramFiles\mihomo-manager"
Copy-Item .\mihomo-manager-windows-amd64.exe "$env:ProgramFiles\mihomo-manager\mihomo-manager.exe"
& "$env:ProgramFiles\mihomo-manager\mihomo-manager.exe" install
& "$env:ProgramFiles\mihomo-manager\mihomo-manager.exe" subscription set 'https://example.com/sub'
& "$env:ProgramFiles\mihomo-manager\mihomo-manager.exe" subscription update
```

ARM64 Windows 使用 `mihomo-manager-windows-arm64.exe`。安装后服务和定时任务会引用 manager 的绝对路径，请保留该文件。需要修改系统状态的命令和 TUI 应从管理员终端运行。

Windows 的核心及配置保存在 `%ProgramData%\mihomo`，状态和日志保存在 `%ProgramData%\mihomo-manager`。`reload` 通过重启核心应用配置，会短暂中断连接；`logs` 读取核心日志并支持 `--tail=N` 和 `--follow`。默认编辑器为 `notepad.exe`，可通过 `$env:EDITOR` 指定其他编辑器。

### Debian/Ubuntu

```bash
sudo dpkg -i mihomo-manager_*_amd64.deb
```

### Arch Linux

```bash
sudo pacman -U mihomo-manager-*-x86_64.pkg.tar.zst
```

### 从源码编译

```bash
go build -o mihomo-manager .
```

### GitHub Actions 编译

推送到 `main`、提交 Pull Request，或在 Actions 页面手动运行 `ci`，会在 Linux 和 Windows 上执行检查，并编译两个平台的 amd64、arm64 二进制。可在对应运行的 **Artifacts** 中下载构建产物，保留 14 天。推送 `v*` 标签会触发 Release 打包和发布。

## 快速开始

```bash
# 安装 mihomo（在线下载）
sudo mihomo-manager install

# 安装 mihomo（从本地 .gz、.zip 或二进制文件）
sudo mihomo-manager install --from ./mihomo-linux-amd64.gz

# 设置订阅
sudo mihomo-manager subscription set https://example.com/sub

# 拉取并应用配置
sudo mihomo-manager subscription update

# 查看状态
mihomo-manager status

# TUI 界面
mihomo-manager
```

## 下载加速

### 代理下载（MIHOMO_DOWNLOAD_PROXY）

当系统设置的 `HTTP_PROXY` 指向 mihomo 自身时，首次安装会陷入先有鸡还是先有蛋的困境。设置 `MIHOMO_DOWNLOAD_PROXY` 可指定一个独立代理专门用于下载 mihomo 核心：

```bash
# 走 SOCKS5 代理下载
export MIHOMO_DOWNLOAD_PROXY=socks5://127.0.0.1:10808
sudo mihomo-manager install

# 走 HTTP 代理下载
export MIHOMO_DOWNLOAD_PROXY=http://127.0.0.1:10809
sudo mihomo-manager install
```

### 镜像加速（MIHOMO_RELEASE_URL）

国内无法直连 GitHub 时，可通过镜像下载。URL 模板支持 `{os}`、`{arch}`、`{version}` 占位符：

```bash
# 使用 ghproxy.com（推荐）
export MIHOMO_RELEASE_URL="https://ghproxy.com/https://github.com/MetaCubeX/mihomo/releases/download/{version}/mihomo-{os}-{arch}-{version}.gz"
sudo mihomo-manager install

# 自建镜像（必须同时提供 checksum 模板）
export MIHOMO_RELEASE_URL="https://cdn.example.com/mihomo/{version}/mihomo-{os}-{arch}-{version}.gz"
export MIHOMO_RELEASE_CHECKSUM_URL="https://cdn.example.com/mihomo/{version}/{asset}.sha256"
sudo mihomo-manager install
```

## 命令

常用命令：

```bash
mihomo-manager install
mihomo-manager status
mihomo-manager subscription set '<url-or-data>'
mihomo-manager subscription update
mihomo-manager config preview
mihomo-manager config override edit
mihomo-manager upgrade
mihomo-manager uninstall
```

完整命令及参数请运行：

```bash
mihomo-manager --help
```

## 配置

最终配置由两部分合并生成：

- **订阅数据**（subscription-data）：订阅 URL 拉取或本地粘贴的内容，作为合并的 base
- **订阅来源**（subscription-source）：显式记录 `remote` 或 `local`；切换来源时清理非活动来源，避免旧 URL 或数据被误用
- **覆写文件**（override-file）：`/opt/mihomo/etc/override.yaml`，本地定制的唯一入口

TUI 的 Config → Subscription 页面支持按 `e` 使用 `$EDITOR` 输入 URL 或本地 subscription-data；CLI 仍可使用 `subscription set '<url-or-data>'`。

合并语义：

- 同名标量/映射：覆写文件**覆盖**订阅值
- 订阅缺失的字段：覆写文件**补充**
- 数组字段（`proxies`、`proxy-groups`、`rules`、`proxy-providers`、`rule-providers`）：默认**追加**到订阅数组末尾；标 `!replace` 时**整体替换**
- 顶层字段写成 `field: !delete null` 时，从生成配置中**删除**；删除标记不会传给核心

合并结果写入 `/opt/mihomo/etc/config.yaml`——**纯生成物**，手动修改会在下次刷新时丢失。
想保留手动修改，运行 `config adopt` 将差异迁移进覆写文件（候选差异 ≥5 字段需 `--force` 确认）。顶层标量/映射的删除会保存为 `!delete`；嵌套键删除会将所在顶层映射保存为 `!replace`，防止深度合并恢复已删除的键。数组差异（包括删除整个数组字段）只报告，不自动采纳。

`subscription update` 会先生成并校验临时配置，再原子替换最终配置；reload 失败或进程在确认重载前中断时，保留生成物并报告 `pending-reload`。事务记录保留到重载结果持久化；预览或校验时恢复记录不会自动重载服务。`status` 和 TUI 状态页显示最近一次配置应用结果（`applied`、`pending-reload`、`validation-failed` 或 `apply-failed`）。

scheduled subscription-update 由 Linux systemd timer 或 Windows 任务计划程序负责，manager CLI 退出后仍会执行；关闭或卸载时会移除 native task。Windows 任务以 SYSTEM 运行，间隔支持 1 小时到 31 天，精确到整秒。

`start`/`restart` 前会自动校验配置，非法配置拒绝启动。

`logs` 在 Linux 使用 `journalctl`，在 Windows 读取 `%ProgramData%\mihomo-manager\logs\mihomo.log`。

## 环境变量

| 变量 | 说明 |
|---|---|
| `MIHOMO_DOWNLOAD_PROXY` | 用于下载 mihomo 核心的代理（绕过系统 HTTP_PROXY） |
| `MIHOMO_RELEASE_URL` | Release 下载 URL 模板，支持 `{os}` `{arch}` `{version}` 占位符；设置后必须提供 checksum 模板 |
| `MIHOMO_RELEASE_CHECKSUM_URL` | 自定义镜像 checksum URL 模板，支持 `{version}` `{asset}` 占位符；缺失或校验失败时拒绝安装 |

## 验收测试

```bash
sudo -E env "PATH=$PATH" go test -tags=acceptance ./acceptance/ -count=1 -v
```

需要 passwordless sudo、systemd、可访问 github.com。

## 许可

GNU GPLv3-or-later

订阅、覆写、生成配置及状态文件按私有权限保存：Linux 文件 `0600`、配置/状态目录 `0700`；Windows 使用 ACL，只允许文件所有者、Administrators 和 SYSTEM 访问。已有文件会在配置操作时收紧权限；目录权限同时保护旧备份。Linux 的 `status` 读取私有应用状态时通过 sudo 提升权限。systemd 单元不包含订阅数据，仍使用 `0644`。

升级在停止核心前持久记录事务与原始二进制 hash。管理器进程中断后，下次安装、升级或卸载会先恢复原二进制和原运行状态；恢复失败时保留事务及备份并报告错误。
