Status: completed

# --quiet 模式 + 日志查看

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

两个不相关的独立功能，合并为一个 slice 因各自太小。

### --quiet 模式

所有 CLI 命令支持 `--quiet` 或 `-q` 标记（紧随子命令之后），抑制非错误的标准输出。

```
mihomo-manager status --quiet   # 只输出状态行，无装饰
mihomo-manager install --quiet  # 不输出进度，出错时只输出错误
```

退出码行为不变，以便脚本使用。

### 日志查看

`mihomo-manager logs` 命令 tail mihomo 的标准输出和标准错误流。

实现方式：
- 读取 `/opt/mihomo/run/mihomo.log`（如果 mihomo 配置了日志文件路径）
- 如果日志文件不存在，尝试通过 `journalctl -u mihomo --follow`（Linux）或对应的系统日志接口查看
- 支持 `--tail N` 显示最后 N 行，`--follow` 持续跟随

## Acceptance criteria

- [ ] 所有 CLI 命令支持 `--quiet` 标记
- [ ] `--quiet` 仅抑制 stdout，不抑制 stderr，不影响退出码
- [ ] `mihomo-manager logs` 可查看 mihomo 日志
- [ ] `logs --tail 100` 显示最后 100 行
- [ ] `logs --follow` 持续跟随输出
- [ ] Manager 模块有测试覆盖日志读取逻辑

## Blocked by

None - can start immediately
