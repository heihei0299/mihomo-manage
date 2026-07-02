# mihomo-manager 验收测试

本文档定义 mihomo-manager 的验收测试用例。每次发布前或修改核心逻辑后，应逐条执行。

## 环境约定

- 操作系统：Linux (systemd) 或 macOS (launchd)
- 测试账户：sudo 权限
- 测试路径：`/opt/mihomo/`、`/opt/mihomo-manager/`
- mihomo 版本：latest
- 网络要求：可访问 `github.com`

---

## AT-01：安装

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 未安装 |
| **命令** | `sudo mihomo-manager install` |
| **通过条件** | 全部满足 |

### 输出文本逐行检查

```
  [fetch] Downloading mihomo latest
  [fetch] Download complete
✓ [deploy] Deploying binary
✓ [bootstrap] Creating config directory
✓ [register] Registering system service
✓ [start] Starting mihomo
mihomo installed successfully
```

每行前缀含义：`  `（空格）= 进行中，`✓` = 完成，`✗` = 失败（不应出现）。

### 退出码

- 成功时退出码为 **0**
- 任何阶段失败时退出码为 **非 0**（此时输出中出现 `✗` 前缀）

### 安装后文件检查

| 路径 | 要求 |
|------|------|
| `/opt/mihomo/bin/mihomo` | 存在，权限包含 execute |
| `/opt/mihomo/etc/config-template.yaml` | 存在，内容包含 `{{subscription}}` |
| `/opt/mihomo/etc/config.yaml` | 存在，内容包含 `port: 7890` |
| `/opt/mihomo-manager/state/subscription-data.txt` | 存在 |

### 服务检查

- `systemctl is-active mihomo` → 输出 `active`

### 失败场景

| 场景 | 预期 |
|------|------|
| 安装时网络不可达 | 输出 `✗ [fetch]`，退出码非 0，`/opt/mihomo/bin/mihomo` 不存在 |
| 安装时无 sudo | 错误信息包含 `permission denied`，退出码非 0 |

---

## AT-02：查看状态

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装且运行中 |
| **命令** | `mihomo-manager status` |
| **通过条件** | 全部满足 |

### 输出文本

```
mihomo: running  (version: v<数字>.<数字>.<数字>)
```

版本号格式：`v` 开头，三段数字，例如 `v1.19.27`。

### 退出码

| 状态 | 退出码 | 输出 |
|------|--------|------|
| running | 0 | `mihomo: running  (version: vX.Y.Z)` |
| stopped / failed | 非 0 | `mihomo: stopped  (version: vX.Y.Z)` |
| not installed | 非 0 | `mihomo: not installed` |

### 其他状态

- `mihomo-manager status`（无 `--quiet`）：正常显示状态行
- `mihomo-manager --quiet status`：无任何输出，仅通过退出码传达状态

---

## AT-03：TUN 网卡确认

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装且运行中；配置中已启用 TUN 模式 |
| **命令** | `ip link show meta` |
| **通过条件** | 全部满足 |

### 检查步骤

1. 确认配置中启用了 TUN：
   ```bash
   grep -q "tun:" /opt/mihomo/etc/config.yaml
   ```
   若 grep 无匹配，则跳过本项（标记为 N/A）。

2. 查询 TUN 网卡：
   ```bash
   ip link show meta
   ```

### 输出文本

```
<meta>: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 ...
```

关键标志：`UP` 和 `LOWER_UP` 必须同时存在。

### 失败场景

| 场景 | 预期 |
|------|------|
| mihomo 配置有 TUN 但网卡不存在 | `ip link show meta` 返回 `Device "meta" does not exist` |
| mihomo 已停止 | meta 网卡可能消失，属正常行为 |

---

## AT-04：systemd 服务确认

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装 |
| **命令** | `systemctl is-active mihomo` |
| **通过条件** | 全部满足 |

### 输出文本

```
active
```

### 其他 systemctl 检查

| 命令 | 通过条件 |
|------|----------|
| `systemctl is-enabled mihomo` | 输出 `enabled`（开机自启已注册） |
| `systemctl status mihomo --no-pager` | 输出包含 `active (running)`，无 `FAILED` 行 |

### 失败场景

| 场景 | 预期 |
|------|------|
| mihomo 未安装 | 退出码 3，输出 `not-found` |
| mihomo 已安装但未启动 | 退出码 1，输出 `inactive` |
| 二进制崩溃后 | 退出码 2，输出 `failed` |

---

## AT-05：停止

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 正在运行（`mihomo-manager status` 退出码 0） |
| **命令** | `mihomo-manager stop` |
| **通过条件** | 全部满足 |

### 输出文本

```
mihomo stopped
```

`--quiet` 模式下无输出。

### 退出码

- 成功：**0**
- 已停止时重复 stop：非 0

### 停止后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 服务状态 | `systemctl is-active mihomo` | 输出 `inactive` |
| 进程 | `pgrep -x mihomo` | 退出码非 0（无进程） |
| 状态命令 | `mihomo-manager status` | 退出码非 0，输出 `mihomo: stopped` |

### 失败场景

| 场景 | 预期 |
|------|------|
| 未安装时 stop | 错误信息，退出码非 0 |
| 已停止时 stop | 错误信息，退出码非 0 |

---

## AT-06：启动

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装但已停止（`mihomo-manager status` 退出码非 0） |
| **命令** | `mihomo-manager start` |
| **通过条件** | 全部满足 |

### 输出文本

```
mihomo started
```

`--quiet` 模式下无输出。

### 退出码

- 成功：**0**
- 已运行时重复 start：非 0

### 启动后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 服务状态 | `systemctl is-active mihomo` | 输出 `active` |
| 进程 | `pgrep -x mihomo` | 退出码 0（进程存在） |
| 状态命令 | `mihomo-manager status` | 退出码 0，输出 `mihomo: running` |

---

## AT-07：重载配置

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 正在运行 |
| **命令** | `mihomo-manager reload` |
| **通过条件** | 全部满足 |

### 输出文本

```
mihomo reloaded
```

### 退出码

- 成功：**0**
- 已停止时 reload：非 0

### 重载前/后对比

```
# 重载前记录 PID
PID_BEFORE=$(pgrep -x mihomo)
mihomo-manager reload
PID_AFTER=$(pgrep -x mihomo)
# PID_AFTER == PID_BEFORE 为重载成功（进程不重启）
```

| 检查项 | 通过条件 |
|--------|----------|
| PID 不变 | `$PID_BEFORE` == `$PID_AFTER` |
| 服务状态 | `systemctl is-active mihomo` → `active` |
| config 生效 | 修改模板后 reload，mihomo 行为反映新配置 |

---

## AT-08：重启

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 正在运行 |
| **命令** | `mihomo-manager restart` |
| **通过条件** | 全部满足 |

### 输出文本

```
mihomo restarted
```

### 退出码

- 成功：**0**
- 已停止时 restart：非 0

### 重启后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 服务状态 | `systemctl is-active mihomo` | 输出 `active` |
| 进程 | `pgrep -x mihomo` | 退出码 0，PID 与重启前不同 |
| 状态命令 | `mihomo-manager status` | 退出码 0，输出 `mihomo: running` |

---

## AT-09：设置订阅源

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装 |
| **命令** | `mihomo-manager subscription set <URL>` |
| **通过条件** | 全部满足 |

### 输出文本

```
subscription saved
```

### 退出码

- 成功：**0**
- 未提供参数：非 0

### 设置后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| URL 文件 | `cat /opt/mihomo-manager/state/subscription-url.txt` | 内容等于输入的 URL |
| 数据文件 | `cat /opt/mihomo-manager/state/subscription-data.txt` | 内容等于输入的 URL（首次 set 写入 data） |

### 设置本地内容（非 URL）

```
mihomo-manager subscription set "proxies:\n  - name: myproxy\n    type: ss\n    server: 1.2.3.4"
```

此时 `subscription-url.txt` 不可读（os.ErrNotExist），`subscription-data.txt` 包含粘贴的内容。

---

## AT-10：订阅更新

| 项目 | 内容 |
|------|------|
| **前置** | 已设置有效订阅 URL |
| **命令** | `mihomo-manager subscription update` |
| **通过条件** | 全部满足 |

### 输出文本

```
config updated
```

`--quiet` 模式下无输出。

### 退出码

- 成功：**0**
- 订阅 URL 不可达：非 0

### 更新后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 生成配置 | `grep -c "proxies" /opt/mihomo/etc/config.yaml` | 输出 > 0（包含代理节点） |
| 备份文件 | `ls /opt/mihomo/etc/config.yaml.bak.*` | 至少存在一个备份 |
| 服务状态 | `systemctl is-active mihomo` | 输出 `active`（reload 后不中断） |

### 更新流程

1. 自动备份当前 config.yaml → `config.yaml.bak.<unix-timestamp>`
2. 从远程拉取最新订阅数据
3. 合并模板 + 新订阅数据 + 分流规则 → 写入 config.yaml
4. 执行 reload 使新配置生效

---

## AT-11：预览配置

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装，有订阅数据 |
| **命令** | `mihomo-manager config preview` |
| **通过条件** | 全部满足 |

### 输出验证

输出为 YAML 格式的完整配置。至少包含：

| 键路径 | 示例值 |
|--------|--------|
| `port` | `7890` |
| `proxies` | 非空列表 |
| `proxy-groups` | 至少包含默认分组 |
| `rules` | 至少包含兜底规则 `MATCH,DIRECT` |

### 退出码

- 成功：**0**
- 无订阅数据时 config preview：输出不含 `proxies` 但退出码仍为 0（预览可能不完整但不报错）

### 合法性检查

```
mihomo-manager config preview | head -5 | grep -q "^port:"
```

---

## AT-12：编辑模板

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装 |
| **命令** | `mihomo-manager template edit` |
| **通过条件** | 全部满足 |

### 行为

1. 打开 `$EDITOR`（默认 `vi`）编辑 `/opt/mihomo/etc/config-template.yaml`
2. 保存并退出编辑器后，自动执行订阅更新
3. 输出 `config updated`

### 退出码

- 编辑后自动更新成功：**0**
- 编辑器返回非零退出码：非 0

### 验证

```
# 编辑前/后对比
md5sum /opt/mihomo/etc/config-template.yaml
# 修改模板文件，保存退出后
mihomo-manager status  # 服务仍正常运行
```

---

## AT-13：编辑分流规则

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装 |
| **命令** | `mihomo-manager rules edit` |
| **通过条件** | 全部满足 |

### 行为

1. 打开 `$EDITOR`（默认 `vi`）编辑 `/opt/mihomo/etc/routing-rules.txt`
2. 保存并退出编辑器后，自动执行订阅更新
3. 输出 `config updated`

### 退出码

- 编辑后自动更新成功：**0**
- 编辑器返回非零退出码：非 0

### 验证

```
# 修改前
cat /opt/mihomo/etc/routing-rules.txt
# 增加一条规则如：- DOMAIN-SUFFIX,example.com,Proxy
mihomo-manager rules edit  # 保存退出
mihomo-manager config preview | grep -q example.com
```

---

## AT-14：版本列表

| 项目 | 内容 |
|------|------|
| **前置** | 网络可访问 GitHub |
| **命令** | `mihomo-manager versions` |
| **通过条件** | 全部满足 |

### 输出格式

每行一个版本标签，共 5 行：

```
v3.0.0
v2.9.6
v2.9.5
v2.9.4
v2.9.3
```

### 退出码

- 成功：**0**
- 无网络时：非 0，错误信息包含网络相关字样

---

## AT-15：升级

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装且运行中 |
| **命令** | `sudo mihomo-manager upgrade <版本>` |
| **通过条件** | 全部满足 |

### 输出文本

```
  [fetch] Downloading mihomo vX.Y.Z
  [fetch] Download complete
✓ [deploy] Deploying binary
✓ [start] Starting mihomo
upgrade complete
```

### 退出码

- 成功：**0**
- 指定版本不存在：非 0

### 升级后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 新版本运行 | `mihomo-manager status` | 退出码 0，版本号为升级目标版本 |
| 旧版本备份 | `ls /opt/mihomo-manager/backups/mihomo.bak` | 文件存在 |
| 服务状态 | `systemctl is-active mihomo` | 输出 `active` |

### 升级到 latest

```
sudo mihomo-manager upgrade
```

不指定版本时升级到最新版，行为同上。

---

## AT-16：升级失败回滚

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装且运行中 |
| **方法** | 替换二进制为一个立即退出的假脚本 |
| **通过条件** | 全部满足 |

### 模拟步骤

```
# 记录当前版本
OLD_VER=$(mihomo-manager status 2>&1 | grep -oP 'v[\d.]+')
# 备份二进制后替换为假脚本
sudo cp /opt/mihomo/bin/mihomo /tmp/mihomo.real
echo -e '#!/bin/sh\nexit 1' | sudo tee /opt/mihomo/bin/mihomo
sudo chmod +x /opt/mihomo/bin/mihomo
# 执行升级（触发 fetch → deploy，新版本启动失败 → 回滚）
sudo mihomo-manager upgrade v9.99.99
```

### 回滚后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 回滚成功 | `mihomo-manager status` | 退出码 0，版本号=`$OLD_VER`（恢复旧版本） |
| 服务状态 | `systemctl is-active mihomo` | 输出 `active` |
| 二进制恢复 | `md5sum /opt/mihomo/bin/mihomo` | 哈希值等于 `/tmp/mihomo.real` |

### 恢复现场

```
sudo cp /tmp/mihomo.real /opt/mihomo/bin/mihomo
```

---

## AT-17：定时订阅刷新

| 项目 | 内容 |
|------|------|
| **前置** | 已设置订阅 URL |
| **命令** | `mihomo-manager subscription schedule --interval 6h` |
| **通过条件** | 全部满足 |

### 输出文本

```
schedule set to every 6h0m0s
```

### 查看/关闭/拒短

| 操作 | 命令 | 通过条件 |
|------|------|----------|
| 查看 | `mihomo-manager subscription schedule` | 输出 `schedule: every 6h0m0s` |
| 关闭 | `mihomo-manager subscription schedule --off` | 输出 `schedule stopped`（`--quiet` 时无输出），退出码 0 |
| 查看关闭后 | `mihomo-manager subscription schedule` | 输出 `schedule: off` |
| 拒短 | `mihomo-manager subscription schedule --interval 30s` | 错误信息，不设置（间隔必须 ≥ 1m） |

### 退出码

- 成功：**0**
- 间隔太短：非 0

---

## AT-18：查看日志

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已运行且有日志输出 |
| **命令** | `mihomo-manager logs --tail=10` |
| **通过条件** | 全部满足 |

### 输出格式

journalctl 格式日志，每条包含时间戳、进程名、消息：

```
Jul 02 10:00:00 host mihomo[12345]: [INFO] ... 
```

### 参数测试

| 命令 | 通过条件 |
|------|----------|
| `mihomo-manager logs --tail=5` | 输出恰好 5 行 |
| `mihomo-manager logs --tail=100` | 输出 100 行（或实际行数，不满 100 则全输出） |
| `mihomo-manager logs --follow -f` | 持续输出新日志，Ctrl+C 退出 |
| `mihomo-manager logs` | 输出默认 50 行 |

### 退出码

- 正常输出：**0**
- mihomo 无日志条目：0（journalctl 返回空但非错误）

---

## AT-19：TUI 仪表盘

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装 |
| **命令** | `mihomo-manager`（无参数） |
| **通过条件** | 全部满足（手动验收） |

### 启动

- TUI 启动，顶部显示当前实例状态（running / stopped / not installed）
- 无报错信息

### 键盘操作

| 按键 | 验证 |
|------|------|
| <kbd>1</kbd> (Start) | 仅当状态为 Stopped 时可触发，触发后进入 Executing 视图 |
| <kbd>2</kbd> (Stop) | 仅当状态为 Running 时可触发 |
| <kbd>3</kbd> (Restart) | 仅当状态为 Running 时可触发 |
| <kbd>4</kbd> (Reload) | 仅当状态为 Running 时可触发 |
| <kbd>i</kbd> (Install) | 仅当未安装时可触发 |
| <kbd>5</kbd> (Upgrade) | 弹出版本选择列表，<kbd>↑</kbd>/<kbd>↓</kbd> 导航，<kbd>Enter</kbd> 确认，显示进度 |
| <kbd>u</kbd> (Uninstall) | 弹出确认对话框，<kbd>y</kbd> 保留备份 / <kbd>n</kbd> 完全删除 / 其他键取消 |
| <kbd>Tab</kbd> | 切换到 Config 视图，显示四个标签页（Subscription / Template / Rules / Preview） |
| <kbd>←</kbd> / <kbd>→</kbd> | 在 Config 四个标签页之间循环切换 |
| <kbd>r</kbd> (Refresh) | 刷新状态显示 |
| <kbd>q</kbd> (Quit) | 退出 TUI，返回 shell |

### Executing 视图

- 触发操作后显示当前阶段进度消息
- 完成后自动返回 Status 视图
- 失败时显示错误信息

### Config 视图

- Subscription 标签页：显示当前订阅 URL/数据
- Template 标签页：显示模板内容
- Rules 标签页：显示分流规则
- Preview 标签页：显示生成的最终配置

---

## AT-20：--quiet 模式

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装且运行中；mihomo 未安装 |
| **命令** | 见下 |
| **通过条件** | 全部满足 |

### 测试

| 命令 | 通过条件 |
|------|----------|
| `mihomo-manager --quiet status`（运行中） | 无 stdout，退出码 0 |
| `mihomo-manager --quiet status`（未安装） | 无 stdout，退出码 2 |
| `mihomo-manager --quiet --version` | 无 stdout，退出码 0（信息写入 stderr 或纯退出码） |
| `mihomo-manager -q status` | `--quiet` 的短格式，行为同上 |

### 错误输出

错误信息始终写入 stderr，`--quiet` 不影响错误信息显示：

```
sudo mihomo-manager --quiet stop 2>/dev/null || echo "exit: $?"  # 成功时无输出
```

---

## AT-21：卸载（不保留备份）

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装 |
| **命令** | `sudo mihomo-manager uninstall` |
| **通过条件** | 全部满足 |

### 输出文本

```
✓ [stop] Stopping mihomo
✓ [unregister] Unregistering system service
✓ [clean] Cleaning up files
mihomo uninstalled
```

### 退出码

- 成功：**0**
- 未安装时 uninstall：非 0

### 卸载后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 二进制 | `ls /opt/mihomo/bin/mihomo 2>/dev/null` | 不存在 |
| 配置目录 | `ls /opt/mihomo/etc/ 2>/dev/null` | 不存在（完全删除） |
| 数据目录 | `ls /opt/mihomo-manager/ 2>/dev/null` | 不存在（完全删除） |
| systemd 服务 | `systemctl list-units --type=service --all \| grep mihomo` | 无匹配 |
| 状态 | `mihomo-manager status` | 退出码 2，输出 `mihomo: not installed` |

---

## AT-22：卸载保留备份

| 项目 | 内容 |
|------|------|
| **前置** | mihomo 已安装 |
| **命令** | `sudo mihomo-manager uninstall --keep-backup` |
| **通过条件** | 全部满足 |

### 输出文本

```
✓ [stop] Stopping mihomo
✓ [unregister] Unregistering system service
✓ [clean] Cleaning up files
mihomo uninstalled
```

### 退出码

- 成功：**0**

### 卸载后验证

| 检查项 | 命令 | 通过条件 |
|--------|------|----------|
| 二进制 | `ls /opt/mihomo/bin/mihomo 2>/dev/null` | 不存在 |
| 配置备份 | `ls /opt/mihomo/etc/config.yaml 2>/dev/null \|\| ls /opt/mihomo/etc/config.yaml.bak.*` | 至少存在一个备份文件 |
| manager 目录 | `ls /opt/mihomo-manager/backups/` | 目录存在且非空 |
| 服务已清理 | `systemctl list-units --type=service --all \| grep mihomo` | 无匹配 |

### 与 AT-21 的唯一区别

AT-22 保留 `/opt/mihomo/etc/config.yaml`（或 `.bak.*`）和 `/opt/mihomo-manager/backups/`；AT-21 删除全部。

---

## 执行检查表

| AT# | 名称 | 结果 | 备注 |
|-----|------|------|------|
| 01 | 安装 | ⬜ | |
| 02 | 查看状态 | ⬜ | |
| 03 | TUN 网卡 | ⬜ | 配置无 TUN 时跳过 |
| 04 | systemd 服务 | ⬜ | |
| 05 | 停止 | ⬜ | |
| 06 | 启动 | ⬜ | |
| 07 | 重载 | ⬜ | |
| 08 | 重启 | ⬜ | |
| 09 | 订阅源 | ⬜ | |
| 10 | 订阅更新 | ⬜ | |
| 11 | 预览配置 | ⬜ | |
| 12 | 编辑模板 | ⬜ | |
| 13 | 编辑规则 | ⬜ | |
| 14 | 版本列表 | ⬜ | |
| 15 | 升级 | ⬜ | |
| 16 | 回滚 | ⬜ | 手动模拟 |
| 17 | 定时刷新 | ⬜ | |
| 18 | 日志 | ⬜ | |
| 19 | TUI | ⬜ | |
| 20 | --quiet | ⬜ | |
| 21 | 卸载 | ⬜ | |
| 22 | 保留备份卸载 | ⬜ | |
