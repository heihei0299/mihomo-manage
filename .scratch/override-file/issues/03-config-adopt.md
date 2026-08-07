# 03 — config adopt：手动修改迁移进覆写文件

**What to build:** `config adopt` 对比当前 config.yaml 与「订阅数据+覆写文件」渲染结果，提取顶层标量/映射字段差异写入覆写文件（保留覆写文件既有内容）；数组字段差异仅在报告中列出、不吸收；候选写入字段 ≥ 5 个判定为大差异，需用户确认（`--force` 跳过确认）；命令幂等（无差异时报告 no changes）。

**Blocked by:** 01（adopt 写入的覆盖字段必须真正生效）、02（写入目标为覆写文件）

**Status:** resolved

- [x] adopt 将 config.yaml 中与渲染结果不一致的标量/映射字段写入覆写文件，覆写文件既有内容保留
- [x] 数组字段差异在报告中列出但不写入
- [x] 无差异时报告 no changes；重复执行幂等
- [x] 候选 ≥5 字段时拒绝执行并展示 diff 报告；`--force` 跳过确认执行
- [x] CLI `config adopt` 命令输出人类可读报告；退出码区分成功/失败

## 实施总结
- 提交：`4b66271` — `feat(core): config adopt 将手动修改迁移进覆写文件`
- 实现的 seams：`ConfigManager.AdoptConfig`（唯一行为 seam）→ `configPipeline.Adopt` + `writeOverrideFields`；CLI `Handler.AdoptConfig` + `config adopt [--force]` 命令
- 验收标准：5/5 全绿（TestAdoptConfigNoExistingConfig / NoChanges / WritesScalarDiff / ArraysReportedNotWritten / Idempotent / LargeDiffNeedsForce + Handler 3 项）
- 测试结果：全绿（go test ./... 三包全过）
- typecheck：go vet ./... 与 go build ./... 通过
- 文档对齐：README CLI 表补 `config adopt [--force]`；usage 帮助同步
- 遗留 / 后续建议：Code Review 修复 2 项（usage 缺 adopt 命令、哨兵错误改用 errors.Is）；CLI 命令更名（template edit → override edit）属 05 票范围
