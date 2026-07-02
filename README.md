# mihomo-manager

mihomo (Clash Meta) 代理管理工具。管理实例的完整生命周期：安装、配置、升级、卸载。

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
sudo dpkg -i mihomo-manager_20260702_amd64.deb
```

### Arch Linux

```bash
sudo pacman -U mihomo-manager-20260702-x86_64.pkg.tar.zst
```

### 从源码编译

```bash
go build -ldflags "-X main.version=$(date +v%Y%m%d)" -o mihomo-manager .
```

## 快速开始

```bash
# 安装 mihomo
sudo mihomo-manager install

# 设置订阅
sudo mihomo-manager subscription set https://example.com/sub

# 拉取并应用配置
sudo mihomo-manager subscription update

# 查看状态
mihomo-manager status

# TUI 界面
mihomo-manager
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
  i [version]       Install mihomo (alias: install)
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
  subscription set string  Set subscription source
  subscription update     Refresh and apply subscription
  subscription schedule [--interval|--off]  View/configure auto-refresh
  template edit     Edit config template ($EDITOR)
  rules edit        Edit routing rules ($EDITOR)
```

## 验收测试

```bash
sudo -E env "PATH=$PATH" go test -tags=acceptance ./acceptance/ -count=1 -v
```

需要 passwordless sudo、systemd、可访问 github.com。

## 许可

MIT
