# Working Memory 内联内容改造计划

**计划ID**: working-memory-inline-content  
**日期**: 2026-06-03  
**状态**: 已实施  
**优先级**: 高  
**作者**: Codex  

## 1. 背景

当前 QukaAI memory 使用 `quka_memory` 作为 agent runtime 记忆目录，并通过 `knowledge_id` 关联 `quka_knowledge` 保存正文、分块和向量。这个设计适合长期 durable memory，因为它可以复用 knowledge 的加密、摘要、chunk、embedding 和 RAG 检索能力。

但 `working memory` 是短生命周期、绑定 runtime context 的临时工作记忆。它通常只需要被当前会话或 agent run hydrate，不需要进入 knowledge 摘要、分块和向量化流程，也不应该表现为用户知识库内容。

## 2. 改造目标

1. 普通 knowledge 创建后仍默认不创建 memory。
2. memory API 创建的 durable backing knowledge 继续使用隐藏 `resource="__memory__"`，并只能通过 memory API 管理。
3. 文档和命名上明确 `knowledge` 是用户知识库，`backing knowledge` 是 durable memory 内部内容背板。
4. `working memory` 支持不创建 backing knowledge，改为在 `quka_memory` 内联保存加密内容。
5. `working memory` 可被 `get / recall / hydrate / pin / delete` 正常使用。
6. durable memory 的行为保持兼容：`core / semantic / episodic` 仍关联 backing knowledge。

## 3. 数据模型

### 3.1 quka_memory 新增字段

新增：

- `title`: working memory 展示标题。
- `content`: working memory 内联正文，存储加密后的文本。
- `content_type`: working memory 内容格式，默认 `markdown`。

### 3.2 knowledge_id 约束调整

将 `knowledge_id` 从强制非空改为可为空字符串：

- durable memory: `knowledge_id <> ''`。
- working memory: 允许 `knowledge_id = ''`，使用内联 `content`。

唯一索引改成 partial unique index：

```sql
CREATE UNIQUE INDEX ... ON quka_memory (knowledge_id) WHERE knowledge_id <> '';
```

这样可以继续保证一个 backing knowledge 最多对应一条 durable memory，同时允许多条 inline working memory。

## 4. 逻辑改造

- `Remember`:
  - 当 `memory_type=working` 且未传 `knowledge_id` 时，不创建 backing knowledge。
  - 直接加密 `args.Content` 写入 `quka_memory.content`。
  - durable memory 仍创建 hidden backing knowledge。
- `Recall`:
  - 先按 metadata 排序与 query/entity_key 做候选筛选。
  - 对有 `knowledge_id` 的 memory 继续走 vector + knowledge 加载。
  - 对 inline working memory 解密 `content`，构造成 runtime-only `Knowledge` 视图返回。
- `Get` / `Hydrate`:
  - 通过统一 helper 把 memory 转成 `MemoryRecallItem`，兼容 backing knowledge 与 inline content。
- `Delete`:
  - inline working memory 删除时只删除 memory/binding/edge，不触碰 knowledge/chunk/vector。
- `Update`:
  - working memory 支持更新 `title/content/content_type`，其中 `content` 会在 logic 层加密后写入。
  - durable memory 不允许通过 memory update 直接修改正文或标题，避免绕过 backing knowledge 的生命周期。
- `Pin`:
  - pin 时更新 `last_accessed_at`，并按当前 `access_count + 1` 递增访问计数。
- `Recall`:
  - inline working memory 会基于 title、entity_key 和解密后的内联正文进入候选，避免只依赖 vector/backing knowledge。

## 5. 非目标

- 本阶段不为 working memory 建立向量索引。
- 本阶段不把 inline working memory 暴露到普通 knowledge API。
- 本阶段不改变 durable memory 的 backing knowledge 模型。

## 6. 风险与验证

- schema migration 需要兼容历史唯一索引。
- 召回逻辑需要避免跳过 `knowledge_id=""` 的 working memory。
- 测试需要覆盖 inline working memory 的 synthetic knowledge 视图、vector options 排除空 knowledge、schema migration partial index。
