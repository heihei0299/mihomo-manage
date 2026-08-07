# 03 — config adopt：手动修改迁移进覆写文件

**What to build:** `config adopt` 对比当前 config.yaml 与「订阅数据+覆写文件」渲染结果，提取顶层标量/映射字段差异写入覆写文件（保留覆写文件既有内容）；数组字段差异仅在报告中列出、不吸收；候选写入字段 ≥ 5 个判定为大差异，需用户确认（`--force` 跳过确认）；命令幂等（无差异时报告 no changes）。

**Blocked by:** 01（adopt 写入的覆盖字段必须真正生效）、02（写入目标为覆写文件）

**Status:** ready-for-agent

- [ ] adopt 将 config.yaml 中与渲染结果不一致的标量/映射字段写入覆写文件，覆写文件既有内容保留
- [ ] 数组字段差异在报告中列出但不写入
- [ ] 无差异时报告 no changes；重复执行幂等
- [ ] 候选 ≥5 字段时拒绝执行并展示 diff 报告；`--force` 跳过确认执行
- [ ] CLI `config adopt` 命令输出人类可读报告；退出码区分成功/失败
