# Journal BlockNote Markdown 转换修复计划

## 问题描述

项目已接入 BlockNote，新的 journal 内容可能以 BlockNote blocks 数组存储。当前 journal tool call 仍直接调用 `editorjs.ConvertEditorJSRawToMarkdown`，当内容不是 EditorJS 原始结构时会转换失败。

## 改造目标

- 保持旧的 EditorJS journal 内容仍可转换为 Markdown。
- 支持新的 BlockNote journal 内容在 tool call 中转换为 Markdown。
- 避免在 journal 业务代码中重复判断内容格式。

## 实施方案

1. 在 `pkg/utils/editorjs` 中新增自动识别 raw 内容格式的 Markdown 转换函数。
2. 顶层 JSON 对象按 EditorJS 处理，顶层 JSON 数组按 BlockNote 处理。
3. 将 journal agent/tool 中直接调用 EditorJS 转换的位置切换为自动识别函数。
4. 补充单元测试覆盖 EditorJS、BlockNote、纯文本三类 journal 内容。

## 相关文件

- `pkg/utils/editorjs/blocknote.go`
- `pkg/utils/editorjs/blocknote_test.go`
- `pkg/ai/agents/journal/function.go`
- `pkg/ai/agents/journal/journal.go`
- `cmd/service/handler/journal.go`

## 状态

- [x] 方案确认
- [x] 代码修改
- [x] 单元测试
