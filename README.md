# mihomo-manager

mihomo (Clash Meta) 代理管理工具。管理实例的完整生命周期：安装、配置、升级、卸载。

当前版本: `v2026.8.7`

## 安装

### 从 Release 下载

从 [Releases](https://github.com/heihei0299/mihomo-manage/releases) 下载对应平台的二进制：

```bash
# Linux amd64
sudo install -m 0755 mihomo-manager-linux-amd64 /usr/local/bin/mihomo-manager

# Linux arm64
sudo install -m 0755 mihomo-manager-linux-arm64 /usr/local/bin/mihomo-manager
```

### Debian/Ubuntu

```bash
sudo dpkg -i mihomo-manager_2026.8.7_amd64.deb
```

### Arch Linux

```bash
sudo pacman -U mihomo-manager-2026.8.7-x86_64.pkg.tar.zst
```

### 从源码编译

```bash
go build -ldflags "-X main.version=v2026.8.7" -o mihomo-manager .
```

## 快速开始

```bash
# 安装 mihomo（在线下载）
sudo mihomo-manager install

# 安装 mihomo（从本地 .gz 或二进制文件）
sudo mihomo-manager install --from ./mihomo-linux-amd64-v1.19.27.gz

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

# 自建镜像
export MIHOMO_RELEASE_URL="https://cdn.example.com/mihomo/{version}/mihomo-{os}-{arch}-{version}.gz"
sudo mihomo-manager install
```

## 命令

```
Usage of mihomo-manager:
  -c                Preview generated config (alias: config preview)
  -h, --help        Show this help
  -q, --quiet       Suppress non-error output
  -s string         Set subscription source (alias: subscription set)
  -t [--interval|--off]  View/configure auto-refresh (alias: subscription schedule)
  -u                Refresh and apply subscription (alias: subscription update)
  -v, --version     Show version
  i/i [ver] [--no-autostart] [--from <path>]  Install mihomo
  ui [--keep-backup]  Uninstall mihomo (alias: uninstall)
  ug [version]      Upgrade mihomo (alias: upgrade)
  v                 List available versions (alias: versions)
  start             Start mihomo
  stop              Stop mihomo
  restart           Restart mihomo
  reload            Reload config
  status            Show mihomo status
  logs [--tail=N] [--follow]  View mihomo logs
  config preview    Preview generated config
  config adopt [--force]  Adopt manual config.yaml changes into override
  subscription set string  Set subscription source
  subscription update     Refresh and apply subscription
  subscription schedule [--interval|--off]  View/configure auto-refresh
  config override edit  Edit override file ($EDITOR)
```

## 配置

最终配置由两部分合并生成：

- **订阅数据**（subscription-data）：订阅 URL 拉取或本地粘贴的内容，作为合并的 base
- **覆写文件**（override-file）：`/opt/mihomo/etc/override.yaml`，本地定制的唯一入口

合并语义：

- 同名标量/映射：覆写文件**覆盖**订阅值
- 订阅缺失的字段：覆写文件**补充**
- 数组字段（`proxies`、`proxy-groups`、`rules`、`proxy-providers`、`rule-providers`）：默认**追加**到订阅数组末尾；标 `!replace` 时**整体替换**

合并结果写入 `/opt/mihomo/etc/config.yaml`——**纯生成物**，手动修改会在下次刷新时丢失。
想保留手动修改，运行 `config adopt` 将差异迁移进覆写文件（候选差异 ≥5 字段需 `--force` 确认）。
`start`/`restart` 前会自动校验配置，非法配置拒绝启动。

## 环境变量

| 变量 | 说明 |
|---|---|
| `MIHOMO_DOWNLOAD_PROXY` | 用于下载 mihomo 核心的代理（绕过系统 HTTP_PROXY） |
| `MIHOMO_RELEASE_URL` | GitHub Release 下载 URL 模板，支持 `{os}` `{arch}` `{version}` 占位符 |

## 验收测试

```bash
sudo -E env "PATH=$PATH" go test -tags=acceptance ./acceptance/ -count=1 -v
```

需要 passwordless sudo、systemd、可访问 github.com。

## 许可

MIT
