# Memory 与 Knowledge 关系说明

**日期**: 2026-05-23  
**状态**: 目标说明文档  
**适用范围**: 当前 QukaAI 后端实现中的 `knowledge`、`memory`、`knowledge_chunk`、`vectors`、`memory_binding`、`memory_edge` 相关逻辑

## 1. 核心结论

在目标模型中，`Knowledge` 是用户知识与 RAG 内容的统一承载层，`Memory` 是面向 agent runtime 的记忆索引和治理层。

更具体地说：

- `Knowledge` 负责保存原始内容、标题、标签、来源、资源分组、处理阶段，并驱动摘要、分块、向量化流程。
- durable `Memory` 不直接保存正文内容，而是通过 `knowledge_id` 指向一条 hidden backing `Knowledge`，再补充记忆类型、作用域、可信度、重要度、来源、实体键、访问统计、运行时绑定和记忆之间的关系。
- `working` memory 可以不创建 backing `Knowledge`，而是在 `quka_memory.content` 中保存加密的短生命周期上下文，用于当前 runtime context 的 hydrate。
- 一条 durable `Memory` 必须依附一条 hidden backing `Knowledge`；但一条 `Knowledge` 不必然拥有对应的 `Memory`。当前数据库层通过 `quka_memory.knowledge_id` 的 partial unique index 保证一个非空 knowledge 最多对应一条 memory，同时允许多条 inline working memory 不绑定 knowledge。
- 普通知识创建后默认只进入知识库、分块、向量化和 RAG 检索；只有被用户、agent 或治理流程显式提升为 agent memory 时，才创建对应 memory。
- 由 durable memory API 自己创建的背板 knowledge 会使用隐藏资源 `__memory__`，不会出现在普通知识列表和知识详情接口中。Inline working memory 不创建 knowledge。

可以把两者理解为：

```text
+-------------------------------+
| Knowledge                     |
|-------------------------------|
| 内容本体                       |
| 原文 / 标题 / 标签 / 来源       |
| 分块 / 向量 / RAG 检索          |
+---------------|---------------+
                |
                | durable memory uses quka_memory.knowledge_id
                v
+-------------------------------+
| Memory                        |
|-------------------------------|
| 记忆目录与治理信息              |
| 类型 / 作用域 / 权限 / 重要度    |
| recall / hydrate / pin / edge  |
+-------------------------------+
                ^
                |
                | working memory may store encrypted inline content
                |
        quka_memory.content
```

## 2. 数据模型关系

### 2.1 Knowledge 相关表

`quka_knowledge` 是知识主表，核心字段包括：

- `id`: knowledge ID。
- `space_id`, `user_id`: 所属空间和作者。
- `resource`: 资源分组，默认是 `knowledge`；durable memory 背板内容使用 `__memory__`。
- `kind`: 内容类型，如 `text`、`image`、`video`、`url`、`chunk`、`rss`。
- `content`, `content_type`: 加密后的正文内容及格式，支持 `markdown`、`html`、`blocks`、`blocks_v2`。
- `title`, `tags`, `maybe_date`: AI 摘要阶段生成或更新的展示和检索信息。
- `stage`: 处理阶段，依次为 `SUMMARIZE`、`EMBEDDING`、`DONE`。
- `source`, `source_ref`: 来源类型和来源引用，如 `chat`、`mcp`、`rss`。
- `expired_at`: 资源周期过期时间，默认不过期。

`quka_knowledge_chunk` 保存 knowledge 摘要分块后的文本片段。`quka_vectors` 保存 chunk 对应的 embedding，并通过 `knowledge_id` 回指 knowledge。

### 2.2 Memory 相关表

`quka_memory` 是记忆目录表，核心字段包括：

- `id`: memory ID。
- `knowledge_id`: durable memory 指向 `quka_knowledge.id`，这是 durable memory 与正文内容之间的主连接；inline working memory 可为空。
- `title`, `content`, `content_type`: working memory 的内联标题和加密正文；durable memory 仍以 backing knowledge 为正文来源。
- `space_id`, `user_id`, `scope`: 决定记忆属于个人、空间，还是用户全局层。
- `memory_type`: 记忆类型，包括 `core`、`episodic`、`semantic`、`working`。
- `status`: 生命周期状态，包括 `active`、`archived`、`superseded`、`deleted`。
- `importance`, `confidence`: 召回排序和治理使用的重要度、可信度。
- `author_type`, `epistemic_status`: 写入主体和认知状态。
- `source_kind`, `source_ref`: 记忆来源，如 `chat`、`mcp`、`rss`、`manual`、`reflection`。
- `entity_key`, `dedupe_key`, `conflict_state`: 实体归档、去重和冲突标记。
- `last_accessed_at`, `access_count`: 召回访问统计。

`quka_memory_binding` 用于把 memory 固定到某个运行时上下文，例如 chat session、agent run、task、workspace。它支持 `pin`、`working`、`hydration_cache` 等绑定类型。

`quka_memory_edge` 用于表示 memory 与 memory 之间的关系，例如 `derived_from`、`supports`、`contradicts`、`related_to`、`supersedes`、`pinned_to`。

## 3. Knowledge 如何运作

### 3.1 创建流程

HTTP 入口是 `POST /:spaceid/knowledge`，对应 `CreateKnowledge` handler，最终进入 `KnowledgeLogic.insertContent`。

创建流程如下：

1. 如果请求未指定 `resource`，默认使用 `knowledge`。
2. 如果内容是 `blocks` 或 `blocks_v2`，先标准化 blocks 中的文件 URL。
3. 对正文内容加密后写入 `quka_knowledge`，初始 `stage` 为 `SUMMARIZE`。
4. 默认不创建 `quka_memory` 记录。
5. 异步或同步进入 knowledge 处理流水线。

这里有一个重要细节：普通 knowledge 是用户知识库内容，不等同于 agent memory。它可以通过显式操作被提升为 memory，例如：

- 用户点击“让 agent 记住”或类似操作。
- agent 在对话中调用 `remember` 并传入已有 `knowledge_id`。
- 某条 knowledge 被 pin 到 runtime context。
- 后台治理任务判断其适合作为长期记忆，并创建对应 memory。

### 3.2 摘要与分块

`KnowledgeProcess.processSummary` 会处理 `SUMMARIZE` 阶段：

1. 读取 knowledge。
2. 将不同 `content_type` 转成 markdown。
3. 调用 AI 的 `Chunk` 能力生成标题、标签、时间和 chunks。
4. 将 chunks 加密写入 `quka_knowledge_chunk`。
5. 更新 knowledge 的 `title`、`tags`、`maybe_date`，并把 `stage` 推进到 `EMBEDDING`。
6. 通过 WebSocket 主题通知前端 stage 变化。

### 3.3 向量化

`KnowledgeProcess.processEmbedding` 会处理 `EMBEDDING` 阶段：

1. 读取 `quka_knowledge_chunk`。
2. 解密 chunk 内容。
3. 调用 embedding 模型生成向量。
4. 删除该 knowledge 旧的 vectors。
5. 批量写入新的 `quka_vectors`。
6. 把 knowledge 的 `stage` 更新为 `DONE`。

Knowledge 的 RAG 检索主要基于 `quka_vectors` 完成：先对 query 做 embedding，再从 vector store 查到 knowledge refs，之后加载 knowledge 内容，必要时进行 rerank，并拼装进 RAG docs。

### 3.4 查询、更新、删除

普通知识列表会自动排除 `resource = "__memory__"` 的内容，因此 memory 背板 knowledge 不会作为普通知识展示。

更新 knowledge 时会重新加密正文，并把 `stage` 重置为 `SUMMARIZE`，随后重新进入摘要和向量化流程。当前代码禁止通过普通 knowledge 更新接口修改 `__memory__` 背板内容。

删除普通 knowledge 时，会同时删除：

- `quka_knowledge`
- `quka_knowledge_chunk`
- `quka_vectors`
- 指向该 knowledge 的 `quka_memory`
- 相关的 `memory_binding` 和 `memory_edge`

## 4. Memory 如何运作

### 4.1 记住：Remember

HTTP 入口是 `POST /:spaceid/memory/remember`。

`MemoryLogic.Remember` 支持两种写入方式：

1. 请求携带已有 `knowledge_id`：memory 直接绑定这条 knowledge。
2. 请求不携带 `knowledge_id` 且不是 `working` memory：系统先创建一条隐藏 knowledge，再创建 memory。
3. 请求不携带 `knowledge_id` 且 `memory_type=working`：系统直接把内容加密写入 `quka_memory.content`，不创建 knowledge。

durable memory 自动创建的 hidden knowledge 使用：

```text
resource = "__memory__"
```

它会调用不自动注册 memory 的 knowledge 创建路径，避免 memory 背板 knowledge 递归创建 memory。

随后 `MemoryLogic.Remember` 会：

1. 对 durable memory 读取对应 knowledge；对 inline working memory 校验内容非空。
2. 对 durable memory 检查是否已经存在绑定该 knowledge 的 memory。
3. 如果存在，则更新重要度、可信度、作者、认知状态、实体键和访问字段。
4. 如果不存在，则创建新的 `quka_memory`。

默认值包括：

- `memory_type`: `semantic`
- `scope`: `user`
- `status`: `active`
- `importance`: `50`
- `confidence`: `0.7`
- `author_type`: `human`
- `epistemic_status`: `stated`
- `source_kind`: `manual`

Agent tool 写入 memory 时会使用更适合对话场景的默认值，例如 `author_type=agent`、`source_kind=chat`、`confidence=0.85`、`importance=70`。

### 4.2 层级与可见性

Memory 通过 `space_id + scope + user_id` 表达访问层：

- `user_global`: `space_id = "-"` 且 `scope = user`，跨空间的用户私有记忆。
- `user_space`: 当前 `space_id` 且 `scope = user`，当前空间内的用户私有记忆。
- `space_shared`: 当前 `space_id` 且 `scope = space`，空间共享记忆。

召回时，系统会同时考虑：

- 当前用户的全局用户记忆。
- 当前用户在当前空间的用户记忆。
- 当前空间的共享记忆。

修改 memory 时，用户私有记忆要求 owner 是当前用户；空间共享记忆要求当前用户在空间内有编辑权限。

### 4.3 召回：Recall

HTTP 入口是 `POST /:spaceid/memory/recall`。

`MemoryLogic.Recall` 的召回流程如下：

1. 先从 `quka_memory` 读取当前用户可访问的 active memories。
2. 如果 query 非空，基于这些 memories 的 `knowledge_id` 限定 vector 查询范围。
3. 对 query 生成 embedding，并在 `quka_vectors` 中查询命中的 knowledge。
4. 按 importance、confidence、updated_at 对候选 memory 排序。
5. 组合候选来源：entity_key 文本命中、vector 命中、排序兜底。
6. 批量加载候选对应的 knowledge，并解密正文。
7. 如果请求 `include_evidence`，额外读取 `memory_edge` 作为证据关系。
8. 返回 memory 元数据和 knowledge 正文。
9. 更新 memory 的 `last_accessed_at` 和 `access_count`。

因此，memory recall 的内容匹配并不是只查 `quka_memory`。`quka_memory` 决定候选池和治理排序；durable memory 的正文检索复用 knowledge 的 chunk/vector 能力，inline working memory 则按内联标题、正文和 entity_key 做轻量匹配，不参与 vector 检索。

### 4.4 上下文装配：Hydrate

HTTP 入口是 `POST /:spaceid/memory/hydrate`。

`MemoryLogic.Hydrate` 用于给运行时装配可直接注入模型上下文的记忆文本：

1. 如果传入 runtime context，先从 `memory_binding` 读取 pinned 或 working memories。
2. 再调用 `Recall` 召回 core、semantic、episodic memories。
3. 合并 working items 和 recall items，去重。
4. 根据 `token_budget` 裁剪。
5. 生成 `assembled_context`，格式类似：

```text
## Working memory: <title>
<content>

## Relevant memory: <title>
<content>
```

如果 runtime context 是 chat session，hydrate 还会同步更新 `chat_session_pin`，保存本次 hydration 的 core、working、recent episodic memory IDs。

### 4.5 固定：Pin

HTTP 入口是 `POST /:spaceid/memory/pin`。

`MemoryLogic.Pin` 会把 memory 固定到某个 runtime context，写入 `quka_memory_binding`。它不会复制正文，只建立 context 到 memory 的绑定关系。后续 hydrate 会优先加载这些 pinned 或 working memories。

### 4.6 反思：Reflect

HTTP 入口是 `POST /:spaceid/memory/reflect`。

`MemoryLogic.Reflect` 用于从运行时上下文中沉淀新的 episodic memory：

- 对 chat session，它读取最近的 chat summary。
- 对 agent run、task、workspace，它读取 `runtime_context.extraction` 中的 summary 或 messages。

反思得到的内容会通过 `Remember` 写成一条 `episodic` memory，并创建一条背板 knowledge。之后，如果该 context 已经绑定了其他 memories，系统会创建 `derived_from` memory edge，表示这条反思记忆来源于那些上下文记忆。

### 4.7 删除：Delete 与 Forget

Memory 删除分软删除和硬删除：

- 软删除：只把 `quka_memory.status` 更新为 `deleted`。
- 硬删除：删除 memory 本身，并删除相关 `memory_binding`、`memory_edge`。

`Forget` 是面向“忘记记忆”的强语义操作。它会硬删除 memory；如果指定 `deleteKnowledge=true`，还会删除背后的 knowledge、chunks 和 vectors，使这条记忆的正文和向量检索结果都不再污染后续上下文。

## 5. 两者的生命周期联动

### 5.1 普通知识创建时

```text
用户创建 Knowledge
  -> 写入 quka_knowledge
  -> KnowledgeProcess 生成 chunks 和 vectors
```

这条路径让普通知识作为知识库卡片被人浏览，并通过 knowledge/RAG 能力被检索。它不会默认进入 memory recall/hydrate 候选池。

### 5.2 将 Knowledge 提升为 Memory 时

```text
用户或 agent 选择已有 Knowledge
  -> 调用 /memory/remember，传入 knowledge_id
  -> 创建 quka_memory 指向该 Knowledge
  -> 后续可被 memory recall / hydrate / pin 使用
```

这条路径适合把用户明确认可、agent 明确需要、或治理流程筛选出的内容纳入 agent runtime 记忆。

### 5.3 Durable Memory 直接创建时

```text
调用 /memory/remember，不传 knowledge_id，且 memory_type 不是 working
  -> 创建 resource="__memory__" 的 hidden Knowledge
  -> 创建 quka_memory 指向该 hidden Knowledge
  -> KnowledgeProcess 继续为 hidden Knowledge 生成 chunks 和 vectors
```

这条路径让 memory 可以复用 knowledge 的内容、分块和向量能力，同时避免把 agent 记忆暴露到普通知识列表。

### 5.4 Working Memory 直接创建时

```text
调用 /memory/remember，不传 knowledge_id，且 memory_type=working
  -> 不创建 Knowledge
  -> 加密内容写入 quka_memory.content
  -> 创建 quka_memory，其中 knowledge_id 为空
  -> 后续可被 pin / hydrate / recall 作为 runtime working context 使用
```

这条路径适合短生命周期的 agent runtime 上下文。它不会触发 knowledge 的摘要、分块和向量化，也不会出现在用户知识库。

### 5.5 删除 Knowledge 时

```text
删除普通 Knowledge
  -> 删除 knowledge / chunks / vectors
  -> 如果存在，找到指向该 knowledge 的 memories
  -> 删除 memories / bindings / edges
```

当前代码禁止通过普通 knowledge API 删除 `__memory__` 背板 knowledge。memory 背板内容应通过 memory delete/forget 入口治理。

### 5.6 忘记 Memory 时

```text
Forget Memory
  -> 删除 memory / bindings / edges
  -> 如果 delete_knowledge=true 且存在 knowledge_id
       -> 删除 backing knowledge / chunks / vectors
  -> 如果是 inline working memory
       -> 不触碰 knowledge / chunks / vectors
```

这条路径是清理 memory 及其内容背板的推荐方式。

## 6. API 职责边界

### 6.1 Knowledge API

Knowledge API 面向用户知识库和 RAG 内容管理：

- 创建、更新、删除、查看知识卡片。
- 展示知识列表。
- 管理长文 chunk 任务关联内容。
- 驱动摘要、分块、向量化。
- 默认隐藏 memory 背板 knowledge。
- 可提供显式“提升为 memory”入口，但不默认把每条 knowledge 变成 memory。

### 6.2 Memory API

Memory API 面向 agent/runtime 的持久记忆：

- `remember`: 写入或更新记忆；durable memory 可创建隐藏背板 knowledge，也可把已有 knowledge 提升为 memory；working memory 可直接内联保存短生命周期上下文。
- `recall`: 根据 query、类型、作用域召回记忆。
- `hydrate`: 为运行时拼装可注入上下文的记忆文本。
- `pin`: 将记忆固定到某个 runtime context。
- `reflect`: 从上下文摘要中沉淀 episodic memory。
- `update/delete`: 治理 memory 元数据和生命周期。

## 7. Agent Tool 层的使用方式

项目中已经封装了 memory agent tools：

- `SearchUserMemories`: 搜索用户持久记忆。
- `RememberUserMemory`: 保存持久记忆。
- `ForgetUserMemory`: 遗忘错误、过时或重复的记忆。

这些 tools 最终仍然调用 `MemoryLogic`。它们对模型暴露的是 memory 语义；对系统内部而言，durable memory 使用 knowledge 背板和 memory 目录组合，working memory 使用 memory 目录中的内联加密内容。

Knowledge tools 则更偏向显式的知识 CRUD，用于让 agent 创建、读取、更新用户知识内容。

## 8. 设计取舍

当前实现的关键取舍是“内容统一，语义分层”：

- durable 正文放在 hidden backing knowledge 体系中，减少重复存储并复用内容处理流水线。
- durable memory 复用 knowledge 的加密、分块、向量化和 RAG 检索能力。
- working memory 使用 `quka_memory` 内联加密内容，不进入 knowledge 处理流水线。
- memory 自己负责记忆治理字段、运行时上下文关系，以及 working memory 的轻量正文。
- hidden resource `__memory__` 让 durable agent memory 不污染普通知识列表；inline working memory 不创建 knowledge。
- 普通知识默认不注册为 memory，避免把资料库全部混入 agent runtime 记忆。
- 需要被 agent 长期使用的知识，通过显式提升、pin、remember 或 reflect 进入 memory。

这种设计带来的注意点：

- 删除语义要区分“删除知识”和“忘记记忆”。前者会清理普通 knowledge 以及可能指向它的 memory；后者可选择是否连同背板 knowledge 一起删除。
- 普通 knowledge API 不允许操作 `__memory__` 背板内容，避免绕过 memory 生命周期。
- durable memory recall 的质量依赖 knowledge 的异步处理是否完成。如果 backing knowledge 还没完成 embedding，召回仍可通过 metadata 排序兜底，但向量命中能力会延迟到 `stage=DONE` 后完整可用。Working memory 不依赖 embedding。
- 当前 `quka_memory.knowledge_id` 是 partial unique index，因此系统语义是一条非空 knowledge 最多对应一条 durable memory，而不是多个不同 durable memory 共享同一条 knowledge。没有 memory 的 knowledge 仍然是合法的普通知识；inline working memory 的 `knowledge_id` 为空。

## 9. 一句话总结

`Knowledge` 是 QukaAI 的用户知识内容底座，负责“存什么、怎么分块、怎么向量化、怎么被 RAG 检索”；`Memory` 是 agent runtime 记忆层，负责“哪些内容应该被当作记忆、谁能看到、何时召回、如何固定到上下文、何时遗忘”。Durable memory 通过 hidden backing knowledge 复用内容流水线；working memory 通过 `quka_memory` 内联内容保持轻量和短生命周期。
