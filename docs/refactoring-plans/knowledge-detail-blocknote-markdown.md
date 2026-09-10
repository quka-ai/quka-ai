# Knowledge 详情 BlockNote Markdown 响应改造计划

## 问题描述和背景

内置 block editor 已经从旧 EditorJS block 切换为 BlockNote。新旧编辑器的原始 block 数据结构不兼容，前端在获取 knowledge 详情时如果遇到旧 `blocks` 内容，需要由服务端转换为 markdown，以便前端使用新编辑器链路展示。

当前服务端已经存在 `editorjs.ConvertKnowledgeRawToMarkdown`，可以根据 `content_type` 将旧 EditorJS (`blocks`) 转换为 markdown。但详情接口此前仅在 app client 场景转换，普通前端请求仍可能拿到旧 blocks JSON。BlockNote (`blocks_v2`) 是当前编辑器使用的数据结构，不需要转换。

## 改造目标

- `GET /:spaceid/knowledge` 详情响应中，遇到旧 `content_type=blocks` 时统一返回 markdown。
- 转换成功后响应中的 `content_type` 改为 `markdown`，`content` 为转换后的 markdown 文本。
- `content_type=blocks_v2` 保持 BlockNote 原始内容响应。
- 转换失败时记录结构化日志，并保留原始内容，避免详情页内容被置空。
- 保持列表和预览接口已有的 markdown 化行为。

## 实施步骤

1. 调整 `cmd/service/handler/knowledge.go` 中的 `KnowledgeToKnowledgeResponse`，移除仅 app client 才转换的限制。
2. 将转换条件收窄为旧 `blocks`，避免误转换当前 BlockNote `blocks_v2`。
3. 移除详情响应中的临时调试输出。
4. 补充单元测试，覆盖 BlockNote `blocks_v2` 在详情响应中保持原始内容。
5. 运行相关 handler/editorjs 测试确认行为。

## 关键考虑点

- `blocks` 表示旧 EditorJS 原始结构，需要转换为 markdown。
- `blocks_v2` 表示 BlockNote 原始 blocks 数组，是当前前端可识别结构，不需要转换。
- `GetKnowledge` logic 层会先对静态资源进行预签名 URL 替换，再进入 handler 转换，所以 markdown 中的资源链接仍应保持可访问。
- 转换失败不应返回空内容，否则会扩大单条异常数据对前端展示的影响。

## 时间线和状态追踪

- 2026-06-25：完成方案设计和最小代码改造。
- 2026-06-25：补充 BlockNote 详情保持原始内容单测。

## 需要确认的问题

- 后续是否需要把 API 契约文档明确为：详情接口仅对旧 `blocks` 返回 `content_type=markdown`，`blocks_v2` 保持原样。

## 相关文件列表

- `cmd/service/handler/knowledge.go`
- `cmd/service/handler/knowledge_test.go`
- `pkg/utils/editorjs/blocknote.go`
