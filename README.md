# mihomo-manager

mihomo（Clash Meta）代理管理工具，支持安装、订阅更新、服务管理和 TUI。

## 支持平台

- Linux amd64、arm64（需要 systemd）
- Windows amd64、arm64（Windows 10 / Server 2016 或更新版本）

macOS 不受支持。各平台二进制和安装包见 [Releases](https://github.com/heihei0299/mihomo-manage/releases)。

## 安装

### Linux

下载对应架构的二进制并安装：

```bash
sudo install -m 0755 mihomo-manager-linux-amd64 /usr/local/bin/mihomo-manager
```

arm64 用户请将文件名换成 `mihomo-manager-linux-arm64`。也可按发行版选择 Release 安装包：

```bash
# Debian / Ubuntu
sudo dpkg -i mihomo-manager_*_amd64.deb

# Arch Linux
sudo pacman -U mihomo-manager-*-x86_64.pkg.tar.zst
```

Debian/Ubuntu arm64 请使用文件名中的 `arm64` 包。

### Windows

在管理员 PowerShell 中将下载的程序放到固定位置并安装：

```powershell
New-Item -ItemType Directory -Force "$env:ProgramFiles\mihomo-manager"
Copy-Item .\mihomo-manager-windows-amd64.exe "$env:ProgramFiles\mihomo-manager\mihomo-manager.exe"
& "$env:ProgramFiles\mihomo-manager\mihomo-manager.exe" install
$manager = "$env:ProgramFiles\mihomo-manager\mihomo-manager.exe"
& $manager subscription set '订阅 URL 或配置内容'
& $manager subscription update
```

arm64 请使用 `mihomo-manager-windows-arm64.exe`。安装后服务会使用该程序路径，请保留文件；管理命令和 TUI 请在管理员终端运行。

## 快速开始（Linux）

```bash
# 安装 mihomo 核心
sudo mihomo-manager install

# 设置订阅并应用配置
sudo mihomo-manager subscription set '订阅 URL 或配置内容'
sudo mihomo-manager subscription update

# 查看状态；不带命令启动 TUI
mihomo-manager status
mihomo-manager
```

## 常用命令

```text
mihomo-manager start
mihomo-manager stop
mihomo-manager restart
mihomo-manager reload
mihomo-manager logs [--tail=N] [--follow]
mihomo-manager upgrade [版本]
mihomo-manager subscription schedule --interval <时长>
mihomo-manager subscription schedule --off
mihomo-manager config preview
mihomo-manager config override edit
mihomo-manager uninstall [--keep-backup]
```

运行 `mihomo-manager --help` 查看完整命令和参数。

## 许可

GNU GPLv3-or-later
