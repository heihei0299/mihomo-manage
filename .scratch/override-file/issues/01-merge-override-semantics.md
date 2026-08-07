# 01 — mergeConfig 升级：任意字段覆盖与 `!replace` 整体替换

**What to build:** 覆写文件（overlay）与订阅数据（base）的合并语义升级：同名标量/映射以覆写文件为准覆盖订阅；订阅缺失字段由覆写文件补充；五个数组字段（`proxies`、`proxy-groups`、`rules`、`proxy-providers`、`rule-providers`）默认追加到订阅数组末尾，标 `!replace` 时整体替换订阅数组。`config preview` 即刻体现新语义。

**Blocked by:** None — 可立即开始

**Status:** resolved

- [x] mergeConfig 纯函数表驱动测试覆盖：同名标量覆盖、同名嵌套映射递归覆盖、订阅缺失字段补充、五个数组默认追加、`!replace` 整体替换、非法 YAML 报错、非 map 覆写回退
- [x] 现有「overlay 不覆盖 base」语义的测试断言全部反转更新
- [x] `config preview` 输出反映新合并语义（订阅字段被覆写文件值覆盖）

## 实施总结
- 提交：`327eb1c` — `feat(core): mergeConfig 支持任意字段覆盖与 !replace 数组整体替换`
- 实现的 seams：`mergeConfig` 纯函数（含 `parseOverlay`/`deepMergeMap`），pipeline 层 preview 经同一 seam 联动
- 验收标准：3/3 全绿（见上方 checkbox；覆盖用例：TestMergeConfigTopLevelScalarOverrides/NestedMapOverrides/ScalarOverrideAndSupplement/ReplaceTag/ReplaceTagEmptyBase/NonAppendArrayOverrides；反转用例：TestPipelineMergeOverridesBaseScalar/OverridesNonAppendArrays 等）
- 测试结果：全绿（go test ./... 三包全过）
- typecheck：go vet ./... 与 go build ./... 通过
- 文档对齐：无需更新（合并语义文档 CONTEXT.md 与 ADR-0007 已在 spec 阶段落盘）
- 遗留 / 后续建议：Code Review 发现的空 base 泄漏 `!replace` 缺陷已修复并有测试保护；路径常量更名属 02 票范围
