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
& $manager subscription sync
```

arm64 请使用 `mihomo-manager-windows-arm64.exe`。安装后服务会使用该程序路径，请保留文件；管理命令和 TUI 请在管理员终端运行。

## 快速开始（Linux）

```bash
# 安装 mihomo 核心
sudo mihomo-manager install

# 设置订阅并应用配置
sudo mihomo-manager subscription set '订阅 URL 或配置内容'
sudo mihomo-manager subscription sync

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
mihomo-manager subscription update
mihomo-manager subscription sync
mihomo-manager config apply
mihomo-manager subscription schedule --interval <时长>
mihomo-manager subscription schedule --off
mihomo-manager config preview
mihomo-manager config override edit
mihomo-manager uninstall [--keep-backup]
```

运行 `mihomo-manager --help` 查看完整命令和参数。

## 覆写文件语义

`override.yaml` 以订阅数据为 base、本地内容为 overlay 合并：

- 映射递归深合并；标量直接覆盖订阅值；订阅缺失的字段直接补充。
- `!replace` 在任意层级整体替换该节点；`!delete` 在任意层级删除该键（挂在集合字段上表示整块移除，如 `proxies: !delete null`），值必须为 `null`。
- `proxies`、`proxy-groups` 按 `name` 对齐，`proxy-providers`、`rule-providers` 按映射键对齐：同名整条替换并保持订阅中的位置，新名字追加到末尾；同名替换不继承订阅字段。
- 数组减项写在数组项上：`- !delete 香港01`（identity-array 按 `name`，identity-map 按键）或 `- !delete DOMAIN-SUFFIX,ads.example.com,DIRECT`（`rules` 按完整规则文本精确匹配）。
- 覆写文件中的 `rules` 插入订阅首个兜底规则（规则类型字段为 `MATCH`，大小写不敏感、忽略两侧空白，如 `MATCH`、`match,DIRECT`、`MATCH ,DIRECT`）之前；订阅没有兜底规则时追加到末尾。
- 删除不存在的键/名字/规则、同一名字既删又增、同一份覆写里重复的映射键或身份条目、`!replace` 子树内再出现 `!delete`、把指令挂在键名上、把指令挂在整个文档上，都会报错，错误信息包含路径（如 `dns.fallback`）。
- 指令标签只在合并期生效，不会写入生成的 `config.yaml`。

升级提示：旧覆写文件中的嵌套指令现在会真正生效；同名节点/策略组/provider 由重复追加变为覆盖；用户规则由订阅 `MATCH` 之后前移到之前。升级后建议先运行 `mihomo-manager config preview` 检查输出变化。

## 许可

GNU GPLv3-or-later
