# OpenClaw Memory 最小适配方案

**计划ID**: openclaw-memory-minimal-adaptation  
**日期**: 2026-03-30  
**状态**: 待审核  
**优先级**: 高  
**作者**: Codex  

**关联文档**:

- [OpenClaw x QukaAI Memory 架构图](/Users/wangboyan/development/quka/quka-ai/docs/refactoring-plans/openclaw-memory-architecture.md)
- [OpenClaw x QukaAI API Adapter Contract](/Users/wangboyan/development/quka/quka-ai/docs/refactoring-plans/openclaw-memory-api-adapter.md)

## 1. 背景

OpenClaw 的记忆设计强调的不是单一 RAG 检索，而是一套面向 agent runtime 的分层记忆系统。其核心能力可以概括为：

- `retain`: 将对话、外部输入、手工记录写入记忆系统
- `recall`: 根据当前任务召回最相关的长期或短期记忆
- `reflect`: 将碎片化交互沉淀为更稳定的长期记忆
- `hydrate`: 在新一轮会话开始时组装一份可直接注入上下文的 memory pack

QukaAI 当前已经具备较好的底层能力：

- `quka_knowledge`: 作为知识正文存储
- `quka_knowledge_chunk`: 作为分块检索载体
- `quka_vectors`: 作为向量索引
- `quka_chat_summary`: 作为会话摘要
- `quka_chat_session_pin`: 作为内置聊天场景下的 pin / hydration cache
- MCP 工具：`create_knowledge`、`get_knowledge`、`search_knowledges`

当前的不足不是“没有存储”，而是现有能力更偏向“知识库 + RAG”，还没有形成 OpenClaw 所需的 “agent memory runtime” 抽象。

本方案的目标是：在尽量复用现有表和逻辑的前提下，以最小改动补齐面向 OpenClaw 风格记忆系统所需的结构和 API。

## 2. 设计目标

### 2.1 核心目标

1. 保留现有 `knowledge/chunk/vector` 体系，不重建正文存储层
2. 支持 `core / episodic / semantic / working` 四类记忆
3. 支持 `remember / recall / reflect / hydrate` 四类核心动作
4. 让 agent 可以自主生成、更新、淘汰和召回记忆
5. 让人可以在界面上看到 agent 可用的 memory 对应 knowledge
6. 让人可以在界面上显式创建、编辑、pin 和纠正知识，作为 agent 的可用记忆输入
7. 支持证据链、来源追踪、时间范围与实体归档
8. 尽量复用现有异步 summarize / embedding 流程
9. 在最小版本中引入 `Mem0` 风格的实用机制：去重、冲突检测、更新删除和重排

### 2.2 非目标

本阶段不包含以下内容：

- 不直接实现 OpenClaw 的本地 Markdown workspace 落盘
- 不强制引入新的全文检索引擎
- 不重做现有聊天主流程
- 不在本阶段实现完整的多 agent 协作语义

## 3. 总体思路

### 3.1 保持 `quka_knowledge` 为 canonical content store

所有记忆正文继续存放在 `quka_knowledge.content` 中，继续沿用现有：

- 加密逻辑
- content type 处理
- chunk 拆分
- embedding 写入
- resource 管理

新增的 memory 层只负责回答两个问题：

1. 这条 knowledge 在 agent memory 体系中扮演什么角色
2. 这条 memory 与哪些会话、实体、证据、其他 memory 存在关系

在当前收敛版本中，knowledge 与 memory 的映射仅通过 `quka_memory.knowledge_id` 表达，不再把 memory 语义字段复制回 `quka_knowledge`。

### 3.2 增加独立 memory catalog

通过单独新增 `quka_memory` 和 `quka_memory_edge` 两张表，在不破坏现有知识库结构的前提下补出记忆抽象。

### 3.3 复用 chat summary 与 runtime context

- `quka_chat_summary` 继续作为 episodic consolidation 的输入
- `quka_chat_session_pin` 仅作为 QukaAI 内置聊天场景下的一种 runtime context 承载体
- 面向外部 agent 时，应使用更通用的 runtime context 语义，而不是强依赖 QukaAI 内部 session

### 3.4 吸收 Mem0 的机制，但不采用其整体架构

本方案不切换为 `Mem0` 式的 memory-first 架构，但吸收其几项非常实用的产品机制：

- 在 `remember` 入口内置记忆抽取、去重和冲突检测
- 将 `update / delete` 视为一等能力，而不是只做 archive
- 在 `recall` 中加入 reranker，避免仅靠向量召回
- 将 `scope` 与 `memory_type` 视为正交维度分别建模

设计取向如下：

- 保留 `knowledge` 作为 canonical store
- 使用 `memory` 作为 agent runtime 层
- 让被提升为 memory 的内容同时服务于 agent runtime 与 human-facing knowledge UI
- 在 runtime 层引入更强的 lifecycle 和 recall 质量控制

### 3.6 惰性激活与渐进治理

本方案不要求用户显式开启 agent mode，也不希望用户为“是否启用 memory runtime”做额外配置。

因此推荐采用：

- 默认不提升
  - 普通 knowledge 创建后只进入知识库、chunk、vector 和 RAG 检索，不自动生成 memory
- 按需提升
  - 用户明确要求“记住”、agent 调用 `remember`、knowledge 被 pin，或后台治理任务判断值得沉淀时，才创建对应 memory
- 惰性激活
  - 只有当 agent 真正开始对某个 space / runtime context 调用 `recall / hydrate / pin` 时，memory 才进入 active runtime usage
- 渐进治理
  - 去重、冲突检测、reflect、merge / supersede 等较重的治理动作，不在所有 knowledge 创建时全量触发，而是在 memory 被创建、使用、命中或周期性后台任务扫描后按需触发

这样可以同时满足三点：

1. 用户无感，不需要做额外配置
2. agent 可以平滑接入，但不会把完整资料库误当作长期记忆
3. 没有 agent 的场景不会承担完整 memory runtime 的重负担

### 3.7 Memory Lifecycle 的维护主体

memory lifecycle 不应由用户手工维护，而应由 agent runtime 与后台任务共同推进。

建议最小版本采用三个内部阶段：

- `registered`
  - knowledge 已被提升为 memory，但尚未被 agent 实际消费
- `activated`
  - memory 已被某个 runtime context 用于 `recall / hydrate / pin`
- `consolidated`
  - memory 已进入更深的治理流程，例如 reflect、merge、supersede、evidence linking

推进方式：

- memory 创建时：系统自动完成 `registered`
- agent 调用 `recall / hydrate / pin` 时：系统将命中的 memory 推进为 `activated`
- 后台任务或 agent reflect 流程运行时：将高价值、高频使用的 memory 推进为 `consolidated`

注意：

- 这是内部 lifecycle，用于治理策略，不要求直接暴露给普通用户
- 用户主要通过 knowledge 间接影响 memory，而不是直接维护这些阶段

### 3.5 面向人的可见性与可控性

QukaAI 的目标不是做一个只有 agent 自己可见的黑盒记忆系统，而是做一个“agent 可自主运转、人可看见并能干预”的 shared memory system。

因此本方案增加两条产品约束：

1. agent 可写入的 memory，原则上都应能映射为人可查看的 knowledge 视图
2. 人在界面上创建或修改的 knowledge，可以被显式提升给 agent 使用，但不应默认进入 memory runtime

这意味着：

- `memory` 是 agent runtime 语义层
- `knowledge` 是人机共享的可视化内容层
- 两者不是二选一，而是同一套系统中的两个视图；memory 是 knowledge 的按需 runtime 投影，不是所有 knowledge 的必然副产物

## 4. 记忆模型

### 4.1 Memory Type

最小适配阶段定义 4 类记忆：

- `core`
  - 稳定、长期、应优先注入上下文
  - 例如用户偏好、长期约束、核心身份信息
- `episodic`
  - 某次对话、某次事件、某段工作过程的沉淀
  - 例如“2026-03-30 讨论了 qukaAI 如何适配 OpenClaw 记忆”
- `semantic`
  - 从 episodic 中抽取出的较稳定事实
  - 例如“用户认为未来 AI 应用会在记忆上发力”
- `working`
  - 当前会话临时工作记忆
  - 例如“本轮任务需要先写设计文档，再实现 schema”

### 4.2 Scope

记忆的生效范围定义为：

- `user`: 用户级长期记忆
- `agent`: agent 自身记忆
- `session`: 当前会话记忆
- `space`: 团队或空间共享记忆

### 4.3 Scope 与 Memory Type 的关系

`scope` 与 `memory_type` 必须视为两个正交维度，不能混用。

- `memory_type` 回答：这条记忆在推理中扮演什么角色
- `scope` 回答：这条记忆对谁生效

示例：

- 一条 `core` memory 可以属于 `user` scope
- 一条 `working` memory 可以属于 `session` scope
- 一条 `semantic` memory 也可以属于 `space` scope

这项拆分是从 `Mem0` 的 scope 设计中借鉴而来，但保留 QukaAI 当前更适合 agent 推理的 memory role 模型。

### 4.4 Relation

memory 之间需要最小关系语义：

- `derived_from`: 某条记忆由其他记忆或事件抽取而来
- `supports`: 证据支持
- `contradicts`: 存在冲突
- `related_to`: 语义相关
- `supersedes`: 新记忆取代旧记忆
- `pinned_to`: 被挂到某个 runtime context working set

### 4.5 Runtime Context

为了兼容 QukaAI 内置聊天与外部 agent runtime，`pin / hydrate / reflect` 不应只绑定到 `session`，而应绑定到更通用的 `runtime_context`。`recall` 保持纯记忆搜索接口，不接收 runtime context；需要装配 pinned / working memories 时使用 `hydrate`。

建议最小版本定义：

- `runtime_context.type`
  - `chat_session`: QukaAI 内置聊天会话
  - `agent_run`: 某次外部 agent 执行
  - `task`: 某个持续存在的任务上下文
  - `workspace`: 某个项目或空间级上下文
- `runtime_context.id`
  - 对应上下文实例 ID

设计原则：

- `chat_session` 只是 runtime context 的一种特例
- 外部 agent 不应依赖 QukaAI 内部 session 才能使用 memory
- `pin` 的目标是 working set，而不是某张具体的 chat 表
### 4.6 Human / Agent / Shared 三类写入主体

为了支持“agent 自主记忆”和“人可视化管理记忆”同时成立，最小版本建议把写入主体作为明确语义保留下来。

建议新增概念：

- `author_type`
  - `human`: 由用户通过 UI 或人工录入创建
  - `agent`: 由 agent 在运行时自动提炼、反思或更新
  - `shared`: 由人发起、agent 整理，或由 agent 生成后经人确认
  - `system`: 由导入、同步、后台任务等系统流程生成

这项信息不改变 `scope` 和 `memory_type` 的定义，而是补充回答：

- 这条记忆是谁写入或确认的
- 这条记忆是否适合直接展示给人
- 这条记忆是否需要更高的 recall / hydrate 权重

示例：

- 用户在界面上手工录入“我偏好中文回复”，则 `author_type = human`
- agent 从长期会话中归纳出“用户偏好结构化输出”，则 `author_type = agent`
- agent 生成后被用户确认的项目约定，则 `author_type = shared`

### 4.7 Epistemic Status

人的表达、agent 的推断、系统的总结，可信度并不相同。为避免系统把“观察”“推断”“总结”“决定”混为一谈，建议为 memory 补充认知状态：

- `stated`: 用户或操作者明确陈述
- `observed`: 从行为或事件直接观测到
- `inferred`: agent 推断得到，未被人确认
- `summarized`: 从对话或日志压缩整理得到
- `confirmed`: 被用户确认过
- `tentative`: 暂时结论，仅供 working memory 使用

这项状态主要影响：

- recall 排序时的可信度权重
- hydrate 时是否直接注入 system prompt/context
- UI 中如何展示“这是用户明确说的”还是“这是 AI 推断的”

## 5. 数据库设计

### 5.1 新增表：`quka_memory`

用途：记忆目录与元信息层。正文仍放在 `quka_knowledge`。

```sql
CREATE TABLE IF NOT EXISTS quka_memory (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    knowledge_id VARCHAR(32) NOT NULL,
    memory_type VARCHAR(20) NOT NULL,
    scope VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    importance SMALLINT NOT NULL DEFAULT 50,
    confidence NUMERIC(4,3) NOT NULL DEFAULT 0.700,
    author_type VARCHAR(20) NOT NULL DEFAULT 'agent',
    epistemic_status VARCHAR(20) NOT NULL DEFAULT 'summarized',
    source_kind VARCHAR(20) NOT NULL,
    source_ref VARCHAR(64) NOT NULL DEFAULT '',
    entity_key VARCHAR(128) NOT NULL DEFAULT '',
    dedupe_key VARCHAR(128) NOT NULL DEFAULT '',
    conflict_state VARCHAR(20) NOT NULL DEFAULT 'none',
    valid_from BIGINT NOT NULL DEFAULT 0,
    valid_to BIGINT NOT NULL DEFAULT 0,
    last_accessed_at BIGINT NOT NULL DEFAULT 0,
    access_count BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quka_memory_knowledge_id
ON quka_memory (knowledge_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_space_type_status
ON quka_memory (space_id, memory_type, status);

CREATE INDEX IF NOT EXISTS idx_quka_memory_entity
ON quka_memory (space_id, entity_key);

CREATE INDEX IF NOT EXISTS idx_quka_memory_dedupe_key
ON quka_memory (space_id, dedupe_key);
```

字段说明：

- `knowledge_id`: 关联现有 `quka_knowledge.id`
- `memory_type`: `core / episodic / semantic / working`
- `scope`: `user / agent / session / space`
- `status`: `active / archived / superseded / deleted`
- `status`: `active / archived / superseded / deleted`
- `importance`: 业务重要度，影响 recall 排序
- `confidence`: 可信度，用于表达“观察”与“推断”的区别
- `author_type`: 写入主体，区分 `human / agent / shared / system`
- `epistemic_status`: 认知状态，区分 `stated / inferred / confirmed` 等
- `source_kind`: `chat / mcp / rss / manual / reflection / import`
- `source_ref`: 来源引用，例如 session ID、订阅 ID、外部导入批次 ID
- `entity_key`: 实体归档键，例如 `user:self`、`project:quka-ai`
- `dedupe_key`: 去重键，用于避免重复记忆膨胀
- `conflict_state`: `none / suspected / confirmed`
- `valid_from / valid_to`: 时间有效范围
- `last_accessed_at / access_count`: 后续 recall 排序和清理策略的基础

补充说明：

- `dedupe_key` 用于在 `remember` 入口快速做近似幂等控制
- `conflict_state` 用于标识该记忆是否与其他记忆冲突，后续可结合 `memory_edge(relation = contradicts)` 做进一步处理
- `registered / activated / consolidated` 建议先作为运行时治理状态存在于逻辑层或缓存层，最小版本不强制落表

### 5.2 新增表：`quka_memory_edge`

用途：记录 memory graph、证据链和反思来源。

```sql
CREATE TABLE IF NOT EXISTS quka_memory_edge (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    from_memory_id VARCHAR(32) NOT NULL,
    to_memory_id VARCHAR(32) NOT NULL,
    relation VARCHAR(20) NOT NULL,
    weight NUMERIC(4,3) NOT NULL DEFAULT 1.000,
    created_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quka_memory_edge_from
ON quka_memory_edge (space_id, from_memory_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_edge_to
ON quka_memory_edge (space_id, to_memory_id);
```

这张表的价值在于：

- 支持 memory 的来源追踪
- 支持冲突关系管理
- 支持从 episodic 反思成 semantic/core 的可解释链路

### 5.3 新增表：`quka_memory_binding`

用途：将 memory 绑定到某个 runtime context，用于 pin、working set 和 hydration cache。

```sql
CREATE TABLE IF NOT EXISTS quka_memory_binding (
    id VARCHAR(32) PRIMARY KEY,
    space_id VARCHAR(32) NOT NULL,
    memory_id VARCHAR(32) NOT NULL,
    context_type VARCHAR(20) NOT NULL,
    context_id VARCHAR(64) NOT NULL,
    binding_type VARCHAR(20) NOT NULL,
    pinned_by VARCHAR(20) NOT NULL DEFAULT 'system',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quka_memory_binding_context
ON quka_memory_binding (space_id, context_type, context_id);

CREATE INDEX IF NOT EXISTS idx_quka_memory_binding_memory
ON quka_memory_binding (space_id, memory_id);
```

字段建议：

- `context_type`: `chat_session / agent_run / task / workspace`
- `context_id`: runtime context 的实例 ID
- `binding_type`: `pin / working / hydration_cache`
- `pinned_by`: `human / agent / system`

设计说明：

- memory 本体不直接归属于某个 session
- memory 与 runtime context 的关系通过 binding 建模
- `quka_chat_session_pin` 可视为 `context_type = chat_session` 的兼容缓存层

### 5.4 对现有表的最小增量

#### 5.4.1 `quka_chat_session_pin`

当前 `content` 仅承载 `knowledges` 和 `journals`。建议扩展为：

```json
{
  "knowledges": [],
  "journals": [],
  "memories": [],
  "hydration": {
    "core": [],
    "working": [],
    "recent_episodic": [],
    "generated_at": 0,
    "version": "v2"
  }
}
```

用途：

- 在内置聊天场景下缓存 `context_type = chat_session` 的 memory working set
- `memories`: 显式 pin 到当前聊天会话的 memory ID
- `hydration`: 存储当前聊天会话的 memory pack 缓存

说明：

- `quka_chat_session_pin` 只服务 QukaAI 内置聊天
- 面向外部 agent 的 pin / hydration 不应直接写这张表，而应优先落在 `quka_memory_binding`

#### 5.4.2 `quka_chat_summary`

最小版本可以不改；如果需要更强可追溯性，建议新增：

```sql
ALTER TABLE quka_chat_summary
ADD COLUMN IF NOT EXISTS summary_type VARCHAR(20) NOT NULL DEFAULT 'rolling',
ADD COLUMN IF NOT EXISTS source_from_sequence BIGINT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS source_to_sequence BIGINT NOT NULL DEFAULT 0;
```

这可以让 chat summary 更接近 episodic slice，而不是单纯滚动摘要。

## 6. API 设计

最小适配阶段仅定义 6 个 API。

### 6.1 `POST /api/v1/memory/remember`

用途：写入记忆。

请求示例：

```json
{
  "content": "用户偏好用中文沟通，输出偏简洁。",
  "title": "Language and response preference",
  "memory_type": "core",
  "scope": "user",
  "author_type": "human",
  "epistemic_status": "stated",
  "entity_key": "user:self",
  "importance": 90,
  "confidence": 0.92,
  "source_kind": "manual",
  "source_ref": ""
}
```

处理流程：

1. 对输入做记忆抽取或标准化
2. 基于 `dedupe_key + semantic similarity + entity_key` 做去重检查
3. 对候选记忆做冲突检测
4. 若命中重复，则执行 `merge` 或 `touch`
5. 若存在冲突，则标记 `conflict_state`
6. 创建或更新 `quka_knowledge`
7. 复用现有异步 summarize / embedding
8. 创建或更新 `quka_memory`
9. 返回 `memory_id` 和 `knowledge_id`

内置策略：

- `dedupe_mode`: `none / touch / merge`
- `conflict_mode`: `ignore / mark / supersede_candidate`

最小版本默认建议：

- `dedupe_mode = merge`
- `conflict_mode = mark`

响应示例：

```json
{
  "memory_id": "mem_xxx",
  "knowledge_id": "kn_xxx",
  "status": "processing",
  "action": "created"
}
```

其中 `action` 可为：

- `created`
- `merged`
- `touched`
- `conflicted`

补充说明：

- 该接口既可由 agent 自动调用，也可由 UI 或导入流程调用
- 如果由人从界面创建，建议默认 `author_type = human`
- 如果由 agent 从聊天中提炼，建议默认 `author_type = agent`
- 在本方案中，新增 knowledge 默认应进入 memory 层，不再要求用户额外勾选是否关联
- knowledge 创建时默认只做轻量登记，不要求同步完成全部重型治理动作

### 6.2 `POST /api/v1/memory/recall`

用途：根据当前任务召回可注入上下文的记忆。

请求示例：

```json
{
  "query": "用户对 OpenClaw 和 AI 记忆的长期观点",
  "limit": 8,
  "scopes": ["user", "space"],
  "memory_types": ["core", "semantic", "episodic"],
  "entity_keys": ["topic:openclaw", "project:quka-ai"],
  "include_evidence": true
}
```

召回策略：

1. 优先取 `core memory`
2. 对 `semantic` 与 `episodic` 走混合检索
3. 对初步召回结果执行 reranker
3. 基于以下维度排序：
   - relevance
   - importance
   - confidence
   - recency
   - access_count

补充说明：

- 初步召回可以复用现有向量检索和后续混合检索方案
- reranker 作为 recall 的独立阶段，避免单靠 embedding 相似度导致记忆误召回
- 如果 recall 命中带 `conflict_state != none` 的记忆，应在返回中显式标记
- `recall` 不读取 runtime context；外部 agent 或内置聊天如需上下文装配，应调用 `hydrate`
- recall 命中的 memory 可被系统内部推进到 `activated` 阶段，并更新访问统计

响应示例：

```json
{
  "items": [
    {
      "memory_id": "mem_1",
      "knowledge_id": "kn_1",
      "memory_type": "core",
      "title": "User preference",
      "content": "用户偏好用中文沟通，输出偏简洁。",
      "confidence": 0.92,
      "importance": 90,
      "evidence": ["mem_9", "mem_10"]
    }
  ],
  "count": 1
}
```

### 6.3 `POST /api/v1/memory/hydrate`

用途：为新会话组装 memory pack。

请求示例：

```json
{
  "runtime_context": {
    "type": "agent_run",
    "id": "run_xxx"
  },
  "query": "当前任务是为 qukaAI 设计适配 OpenClaw 的记忆层",
  "token_budget": 3000
}
```

返回内容应包括：

- `core_memories`
- `working_memories`
- `recent_episodic_memories`
- `semantic_memories`
- `assembled_context`

说明：

- 这里的“新会话”是广义 runtime context，不限于 QukaAI 内置聊天 session
- 对内置聊天，可使用 `runtime_context.type = chat_session`
- 对外部 agent，可使用 `agent_run / task / workspace`

响应示例：

```json
{
  "core_memories": ["mem_1", "mem_2"],
  "working_memories": ["mem_3"],
  "recent_episodic_memories": ["mem_4"],
  "semantic_memories": ["mem_5"],
  "assembled_context": "..."
}
```

### 6.4 `POST /api/v1/memory/reflect`

用途：将聊天或日志沉淀为更稳定的记忆。

请求示例：

```json
{
  "runtime_context": {
    "type": "chat_session",
    "id": "session_xxx"
  },
  "from_sequence": 120,
  "to_sequence": 160,
  "mode": "promote"
}
```

处理流程：

1. 读取 `chat_message`、`chat_summary`
2. 生成候选 episodic memory
3. 根据规则提升为 semantic 或 core
4. 写入 `quka_memory_edge(relation = derived_from)`

说明：

- `reflect` 是 memory lifecycle 从 `activated` 走向 `consolidated` 的关键机制之一
- 可由 agent 主动触发，也可由后台任务在低优先级队列中异步触发

### 6.5 `POST /api/v1/memory/pin`

用途：将记忆 pin 到某个 runtime context。

请求示例：

```json
{
  "runtime_context": {
    "type": "agent_run",
    "id": "run_xxx"
  },
  "binding_type": "pin",
  "memory_ids": ["mem_1", "mem_2"]
}
```

行为：

- 写入 `quka_memory_binding`
- 如果 `runtime_context.type = chat_session`，可同步更新 `quka_chat_session_pin.content.memories` 作为缓存
- 被 pin 的 memory 默认视为已进入 `activated` 阶段

### 6.6 `POST /api/v1/memory/forget`

用途：归档、替换或删除记忆。

请求示例：

```json
{
  "memory_id": "mem_1",
  "mode": "archive"
}
```

支持模式：

- `archive`
- `supersede`
- `delete`

### 6.7 `POST /api/v1/memory/update`

用途：显式更新已有记忆，而不是重复新增。

请求示例：

```json
{
  "memory_id": "mem_1",
  "title": "Updated user preference",
  "content": "用户偏好用中文沟通，输出简洁，但在架构设计上希望保留足够细节。",
  "importance": 95,
  "confidence": 0.95
}
```

处理原则：

1. 优先更新已有 memory 对应的 canonical knowledge
2. 若更新内容与旧版本形成语义替代关系，则创建 `supersedes` edge
3. 保留必要的 source 和 evidence 信息，避免直接覆盖来源

### 6.8 `POST /api/v1/memory/delete`

用途：将删除语义从 `forget` 中显式拆出，作为一等生命周期能力。

请求示例：

```json
{
  "memory_id": "mem_1",
  "hard": false
}
```

处理原则：

- `hard = false`: 逻辑删除，状态改为 `deleted`
- `hard = true`: 真正删除 memory record；对应 knowledge 是否物理删除需遵循现有知识库删除策略

说明：

- 最小阶段可以在 handler 内部复用 `forget(mode=delete)` 的逻辑
- 但 API 语义上应保留显式 `update/delete`，以便兼容更通用的 memory client 使用方式

## 7. 人机双向交互设计

### 7.1 核心产品原则

QukaAI 中的 memory 系统需要满足以下双向关系：

- agent 能主动写 memory，但不能变成黑盒
- 人能看到 agent 在用什么 knowledge 作为记忆依据
- 人能主动创建 knowledge，并选择是否提升给 agent 使用
- 人能纠正 agent 的错误记忆，而不是只能等待模型自行漂移

### 7.2 UI 视图建议

正式用户界面应以 knowledge 为主体，而不是直接暴露 memory runtime 操作面板。

建议最小版本提供：

- `Knowledge List`
  - 面向普通用户的知识列表视图
  - 展示标题、摘要、来源、更新时间、是否已被 agent 纳入记忆
- `Knowledge Detail`
  - 展示 knowledge 正文
  - 附带展示少量 memory 映射信息，例如 memory type、来源、是否被 agent 使用

对于 `Memory Inspector` 和 `Runtime Working Set`：

- 建议仅作为内部调试视图或高级模式能力
- 不作为普通用户的主交互入口

### 7.3 UI 可执行操作

建议普通用户界面只支持 knowledge 层操作：

- 创建 knowledge；默认只进入知识库
- 将某条 knowledge 显式提升为 memory
- 编辑 knowledge 内容
- 查看该 knowledge 是否已被 agent 使用
- 查看该 knowledge 对应的部分 memory 属性

对于以下能力：

- 手动 pin memory
- 直接确认 / 删除 memory
- 操作 runtime working set

建议先保留为内部调试能力，而不是默认开放给普通用户

### 7.4 Human-in-the-loop 原则

不是所有 agent 生成的记忆都应自动成为高优先级长期记忆。建议遵循：

- `human stated` 的 `core memory` 可以直接高权重参与 recall
- `agent inferred` 的长期偏好类记忆，默认应低一档，或进入待确认状态
- 用户主要通过编辑 knowledge 来纠正 agent 的认知
- memory 的直接确认 / 删除操作，最小版本优先保留给内部治理接口

### 7.5 为什么坚持 knowledge 作为 UI 主体

原因不是沿用旧系统，而是因为它天然适合做人机共享层：

- knowledge 有正文，适合阅读、编辑、审核
- memory 更像 runtime metadata，适合 recall、排序、生命周期管理
- 因此 UI 主体应以 knowledge 为中心，memory 作为解释层和行为层附着其上

## 8. MCP / Agent Tools 设计

当前已有的 MCP 工具：

- `create_knowledge`
- `get_knowledge`
- `search_knowledges`

最小适配阶段建议新增以下工具，而不是替换现有工具：

- `remember_memory`
- `recall_memory`
- `pin_memory`
- `reflect_memory`
- `update_memory`
- `delete_memory`

原因：

1. 对 agent runtime 来说，memory 语义比 knowledge 语义更明确
2. 可以保留现有知识库工具的兼容性
3. OpenClaw 或其他 agent runtime 更容易直接对接这组行为接口
4. 也更接近 `Mem0` 风格的通用 memory CRUD 能力

### 8.1 面向外部 Agent 的 API 结构

为了兼容 OpenClaw 或其他外部 agent runtime，memory API 不应要求调用方持有 QukaAI 内部 chat session。

建议所有 runtime 相关 API 统一采用以下上下文结构：

```json
{
  "runtime_context": {
    "type": "agent_run",
    "id": "run_xxx"
  }
}
```

其中：

- `type = chat_session`
  - QukaAI 内置聊天
- `type = agent_run`
  - 某次 agent 执行
- `type = task`
  - 某个持续任务
- `type = workspace`
  - 某个长期项目或空间上下文

最小阶段推荐：

- `remember`
  - 不强依赖 `runtime_context`
- `recall / hydrate / pin`
  - 支持接收 `runtime_context`
- `reflect`
  - 在内置聊天场景使用 `chat_session`
  - 在外部 agent 场景可扩展为基于 run log / event log 做反思

## 9. 与现有模块的映射关系

### 9.1 存储层映射

- `quka_knowledge`
  - 存正文
- `quka_knowledge_chunk`
  - 存 recall 片段
- `quka_vectors`
  - 存 dense retrieval 索引
- `quka_chat_summary`
  - 存会话摘要，作为 episodic consolidation 原料
- `quka_chat_session_pin`
  - 存内置聊天场景下的 pin 与 hydration cache
- `quka_memory`
  - 存记忆元信息
- `quka_memory_edge`
  - 存记忆图关系与证据链
- `quka_memory_binding`
  - 存 memory 与 runtime context 的绑定关系

### 9.2 流程复用

以下能力直接复用：

- `KnowledgeLogic.InsertContentAsyncWithSource`
- 现有 knowledge summarize 流程
- 现有 embedding 生成与向量写入流程
- 现有 `search_knowledges` 背后的 RAG 检索链路

## 10. 最小工作流

建议第一阶段只支持以下工作流：

1. 用户聊天、MCP 输入、人工创建内容
2. 写入 `quka_knowledge`
3. 默认同步或异步写入对应的 `quka_memory`
4. 异步完成 chunk / embedding
5. 在合适时机触发 `reflect`
6. 新的 runtime context 开始时调用 `hydrate`
7. agent 回复阶段调用 `recall`
8. 人可在 UI 上通过 knowledge 查看这些记忆的映射结果，并通过编辑 knowledge 间接影响 memory

这样可以先完成一个“可写入、可召回、可压缩、可注入”的最小 memory runtime。

同时，`remember` 必须承担一部分以往只有异步流程才承担的职责：

- 入口去重
- 冲突标记
- touch / merge 决策

这样可以避免 memory 膨胀过快。

## 11. 实施顺序

### Phase 1

1. 新增 `quka_memory`
2. 新增 `quka_memory_edge`
3. 新增 `quka_memory_binding`
4. 扩展 `quka_chat_session_pin.content` 作为内置聊天兼容缓存
5. 为 `quka_memory` 增加 `author_type / epistemic_status`
6. 新增 `remember / recall / hydrate / update / delete / pin` API，其中 `hydrate / pin / reflect` 接收 `runtime_context`
7. 在 `remember` 中加入去重与冲突标记
8. 在 `recall` 中加入 reranker 接口位
9. 在 UI 侧增加 knowledge 与 memory 的映射展示能力
10. 增加 lifecycle 推进逻辑：`registered -> activated`

### Phase 2

1. 新增 `reflect` API
2. 补充 memory edge 写入逻辑
3. 把 `chat_summary` 更明确纳入 episodic consolidation
4. 增强 `merge / supersede` 规则
5. 增加 lifecycle 推进逻辑：`activated -> consolidated`
6. 打通外部 agent runtime 的 context binding

### Phase 3

1. 将 recall 接入混合检索
2. 增加时间维度召回
3. 增加 entity page 和 memory export/import adapter
4. 增加可视化 memory inspector 和 runtime working set 调试视图

## 12. 风险与注意事项

### 12.1 语义边界风险

如果没有明确区分：

- 观察到的事实
- AI 推断出的偏好
- 暂时性的 working context

则 memory 很容易退化成普通文档集合，影响 recall 精度和可信度。

如果没有进一步区分：

- 用户明确说过的内容
- agent 自行推断的内容
- 双方确认过的内容

则“人的记忆”和“agent 的猜测”会在 UI 和 recall 中被混淆，最终降低用户信任。

### 12.2 数据膨胀风险

如果每轮会话都直接生成大量 episodic memory，长期会带来：

- 检索召回噪音增加
- embedding 成本上升
- hydration 变慢

因此需要在 `reflect` 阶段设置提升与压缩规则。

除此之外，还需要在 `remember` 入口前移治理：

- 重复记忆直接 `touch` 或 `merge`
- 冲突记忆先标记，不立即强行覆盖
- 高频低价值记忆不进入 `core`

### 12.3 向后兼容

本方案默认：

- 不破坏现有知识库 API
- 不改变现有 `search_knowledges` 行为
- memory 能力通过增量表和增量接口叠加
- UI 仍可以先以 knowledge 为主，不要求一次性把所有 memory 元信息都前端化

### 12.4 复杂度控制

引入 `Mem0` 风格机制后，复杂度会明显上升，因此最小版本需要控制边界：

- 去重先做轻量实现：`dedupe_key + embedding similarity`
- 冲突检测先做启发式规则，不在第一阶段引入复杂判定链
- reranker 先预留接口位，允许按 provider 能力渐进接入
- human-in-the-loop 先做最小闭环：可查看、可确认、可删除，不一次性做复杂审批流

## 13. 与 OpenClaw 兼容性评估

### 13.1 结论先行

按当前方案实现后，QukaAI 可以较好地替代 OpenClaw 记忆系统的核心能力，但还不能算对 OpenClaw 当前记忆形态的“原生无缝替换”。

更准确地说：

- QukaAI 可以成为一个更强的 structured memory runtime / backend
- 但如果希望优雅替代 OpenClaw 当前的记忆工作方式，还需要补 agent skill adapter / projection

### 13.2 已可替代的部分

从能力模型看，当前方案已经覆盖 OpenClaw memory 的关键动作：

- `retain`
  - 对应 `knowledge -> memory` 的默认写入，以及 `remember`
- `recall`
  - 对应 `recall`
- `reflect`
  - 对应 `reflect`
- `hydrate`
  - 对应 `hydrate`

并且在以下方面比 OpenClaw 原生方案更强：

- 有独立的 `memory_type / scope / status / importance / confidence`
- 有 `author_type / epistemic_status`
- 有 `quka_memory_edge` 记录证据链和冲突关系
- 有 `quka_memory_binding` 支持 runtime context
- 同时支持 human-facing knowledge UI 和 external agent runtime

因此，如果目标是“承接 OpenClaw 对记忆能力的需求”，当前设计已经足够成立。

### 13.3 还不能直接无缝替换的部分

OpenClaw 当前公开的记忆工作方式有两个明显特征：

1. Markdown workspace 是主要的可见形态和操作界面
2. 常见访问语义是围绕 memory 文件和搜索工具展开，而不是结构化数据库 API

相比之下，QukaAI 当前方案的 source of truth 是：

- `quka_knowledge`
- `quka_memory`
- `quka_memory_edge`
- `quka_memory_binding`

因此两者的差异不在“有没有 retain/recall”，而在“记忆长什么样、怎么被访问”。

主要不兼容点如下：

- OpenClaw 以 Markdown 文件作为主要记忆载体，QukaAI 以 DB 中的 knowledge/memory 作为 canonical store
- OpenClaw 偏向 `memory_search / memory_get` 这类工具语义，QukaAI 偏向 `remember / recall / reflect / hydrate`
- OpenClaw 的 daily log 和 curated long-term memory 仍是显式工作形态，QukaAI 当前方案还没有把它们投影成等价视图
- OpenClaw 的可编辑性天然来自文件系统，QukaAI 的可编辑性主要来自 UI 和结构化数据层

### 13.4 为什么说“能力可替代，形态未完全替代”

当前方案已经能满足 OpenClaw 记忆系统背后的本质目标：

- 写入长期和短期记忆
- 按任务召回
- 做反思沉淀
- 在新 runtime context 开始时装配上下文

但 OpenClaw 用户习惯的并不仅是这些能力，还包括一套很具体的工作形态：

- 通过 workspace 文件直接查看记忆
- 通过 daily log 累积过程
- 通过 curated memory 文件维护长期知识

而当前 QukaAI 方案更像：

- “structured memory operating system”
- 而不是“Markdown-first memory workspace”

所以它已经足以替代 OpenClaw 的 memory engine，但还不是 OpenClaw 的 memory UX / form factor。

### 13.5 为了优雅替代，还需要什么

如果希望 QukaAI 不只是“能接住 OpenClaw 的记忆需求”，而是真正优雅替代其记忆系统，建议新增 agent skill adapter：

#### 13.5.1 Skill Adapter

对外暴露与 OpenClaw 习惯接近的工具语义：

- `memory_search`
  - 内部映射到 `recall`
- `memory_get`
  - 通过 `/memory/get` 读取某条 memory 对应的 canonical content 或 projection view

这样外部 agent runtime 无需理解 QukaAI 全部内部结构，也无需服务端新增 OpenClaw 专用兼容入口，就能接入。

#### 13.5.2 Markdown Projection

即使不把 Markdown 作为 source of truth，也建议提供等价投影视图：

- `MEMORY.md`
  - 对应 `core + confirmed semantic`
- `memory/YYYY-MM-DD.md`
  - 对应某日的 `episodic / working` 视图

这层 projection 的作用不是回退到文件驱动，而是：

- 兼容 OpenClaw 的阅读和调试习惯
- 提供更强的可解释性
- 降低迁移成本

#### 13.5.3 Reflect Mapping

建议把 `reflect` 更明确地对齐到 OpenClaw 风格的两层沉淀：

- `episodic -> daily memory view`
- `semantic/core -> curated long-term memory view`

这样 QukaAI 在认知工作流上就会更接近 OpenClaw，而不是只在底层结构上相似。

### 13.6 最终判断

因此，当前设计文档实现后，可以得出如下判断：

- 作为 memory capability replacement：可以
- 作为 structured backend for OpenClaw-style runtime：可以，而且更强
- 作为 OpenClaw current memory form 的原生直接替代：还不够
- 如果补上 skill adapter + markdown projection + reflect mapping：可以比较优雅地取代

这意味着当前方案的方向是对的，且基础已经足够好；后续的关键不是重做 memory schema，而是在 agent skill / 文档层提供面向 OpenClaw 工作方式的语义映射。

## 14. 落地计划

### 14.1 目标

落地计划的目标不是一次性做完所有 memory 能力，而是分阶段完成三个结果：

1. QukaAI 内部先具备可写入、可召回、可管理的 structured memory runtime
2. human-facing knowledge UI 能看到并管理已提升的 memory
3. 在此基础上补出适配 OpenClaw 工作方式的 skill adapter / projection

### 14.2 总体节奏

建议按四个阶段推进：

1. Phase A: 数据与领域模型落地
2. Phase B: Memory Runtime API 落地
3. Phase C: Human-facing UI 与治理能力落地
4. Phase D: OpenClaw Skill Adapter 与 Markdown Projection 落地

建议执行原则：

- 先做 QukaAI 自己的 source of truth
- 再做 recall / hydrate 的 runtime 闭环
- 再做 UI 可见与人工纠偏
- 最后做 OpenClaw skill adapter 与可选 projection

### 14.3 Phase A: 数据与领域模型

目标：

- 完成 memory 相关表结构、类型和基础 store
- 明确 memory 必须依附 knowledge，但 knowledge 不必然拥有 memory

开发项：

1. 新增 `quka_memory`
2. 新增 `quka_memory_edge`
3. 新增 `quka_memory_binding`
4. 扩展 `quka_knowledge`
5. 扩展 `quka_chat_session_pin.content`
6. 新增 `pkg/types/memory.go`
7. 新增 store interface 和 sqlstore 实现

建议涉及文件：

- `pkg/types/knowledge.go`
- `pkg/types/chat_session_pin.go`
- `pkg/types/tables.go`
- `app/store/store.go`
- `app/store/sqlstore/provider.go`
- `app/store/sqlstore/migrations/*.sql`
- `app/store/sqlstore/knowledge.go`
- `app/store/sqlstore/chat_session_pin.go`
- `app/store/sqlstore/memory.go`（新增）
- `app/store/sqlstore/memory_edge.go`（新增）
- `app/store/sqlstore/memory_binding.go`（新增）

交付物：

- migration 可执行
- types / store 编译通过
- 能通过 store 层完成 memory 的 CRUD 和 binding 的 CRUD

验收标准：

- 可以把一条已有 knowledge 提升为对应 memory
- 能为某条 memory 创建 `chat_session / agent_run / task / workspace` binding
- 内置聊天场景下 `chat_session_pin` 可继续正常工作

### 14.4 Phase B: Memory Runtime API

目标：

- 跑通 memory 作为 runtime 层的最小闭环
- 让 external agent 和 internal chat 都能使用统一 API
- 让 lifecycle 可由 agent runtime 自动推进，而不是依赖用户维护

开发项：

1. `POST /api/v1/memory/remember`
2. `POST /api/v1/memory/recall`
3. `POST /api/v1/memory/hydrate`
4. `POST /api/v1/memory/pin`
5. `POST /api/v1/memory/update`
6. `POST /api/v1/memory/delete`
7. 第一版去重逻辑
8. 第一版冲突标记逻辑
9. `runtime_context` 请求结构

建议涉及文件：

- `cmd/service/handler/memory.go`（新增）
- `app/logic/v1/memory.go`（新增）
- `app/logic/v1/knowledge.go`
- `cmd/service/router/...` 或对应路由注册文件
- `pkg/types/memory.go`

关键实现要求：

- `remember` 可由 knowledge 创建流程自动触发
- `recall` 保持纯搜索，不接收 `runtime_context`
- `hydrate` 接收 `runtime_context`，负责装配 pinned / working memories
- `pin` 写入 `quka_memory_binding`
- `chat_session` 场景下同步更新 `quka_chat_session_pin` 作为缓存
- `recall / hydrate / pin` 命中的 memory 自动推进到 `activated`

交付物：

- 一组可供 internal chat 和 external agent 共用的 memory API

验收标准：

- 创建普通 knowledge 后，不默认生成 memory
- 传入已有 `knowledge_id` 调用 `remember` 时，可以生成对应 memory
- agent_run 场景可以成功 `recall / hydrate / pin`
- chat_session 场景可以继续使用缓存 working set
- recall 返回结果包含 memory metadata
- memory 的访问统计和 activated 状态能够被自动推进

### 14.5 Phase C: Human-facing UI 与治理能力

目标：

- 人能看到 agent 在用哪些 memory
- 人能修改、确认、删除、pin 这些 memory

开发项：

1. knowledge 列表中展示 memory 映射状态
2. knowledge 详情中展示 memory metadata
3. memory inspector 视图
4. runtime working set 视图
5. 用户确认 `agent inferred` memory
6. 用户归档 / 删除 memory
7. 用户手动 pin 到某个 runtime context

后端补充项：

1. knowledge response 增加 memory 映射字段
2. 增加查询某条 knowledge 对应 memory 的接口
3. 增加查询某个 runtime context working set 的接口

交付物：

- UI 可查看 memory
- UI 可管理 memory

验收标准：

- 用户能在 knowledge 详情看到 `memory_type / scope / confidence / author_type / epistemic_status`
- 用户能把 agent inferred memory 标记为 confirmed
- 用户的删除/归档操作会影响后续 hydrate / recall

### 14.6 Phase D: Reflect 与 Consolidation

目标：

- 将碎片化交互沉淀成更稳定的长期记忆
- 控制 episodic 膨胀
- 让 consolidated 阶段由 agent reflect 和后台任务共同维护

开发项：

1. `POST /api/v1/memory/reflect`
2. 读取 `chat_summary`、chat/event log
3. 生成 episodic memory
4. 提升 semantic/core
5. 写入 `quka_memory_edge`
6. 对 confirmed memory 提高权重

交付物：

- 一套可持续运行的 consolidation 机制

验收标准：

- 可从一段聊天或 agent run log 中生成 episodic memory
- 可从 episodic 提升出 semantic/core
- evidence chain 可追溯
- consolidated memory 可以被稳定地产生和复用

### 14.7 Phase E: OpenClaw Skill Adapter

目标：

- 让 QukaAI memory backend 可被 OpenClaw 工作方式自然消费

开发项：

1. `memory_search` skill mapping
2. `memory_get` skill mapping
3. `MEMORY.md` projection
4. `memory/YYYY-MM-DD.md` projection
5. reflect 结果映射到 daily / curated 视图

建议新增模块：

- OpenClaw adapter skill：负责 `memory_search / memory_get / memory_flush` 到原生 `/memory/*` API 的语义映射
- `app/logic/v1/openclaw_memory_projection.go`：仅在需要 Markdown projection 时再考虑

设计要求：

- projection 不作为 source of truth
- source of truth 仍是 `knowledge + memory`
- projection 只用于兼容 OpenClaw 的使用方式、调试和迁移
- 不新增 OpenClaw 专用兼容 HTTP 入口；agent 通过 skill 直接调用 QukaAI 原生 memory API

交付物：

- OpenClaw-facing skill contract
- Markdown-compatible memory views

验收标准：

- 外部 agent 可通过 skill adapter 完成 search/get
- 可导出 `MEMORY.md` 和 daily memory files
- projection 内容与 DB 中 memory 状态一致

### 14.8 建议里程碑

建议按三个里程碑验收：

#### Milestone 1: Internal Memory Runtime

范围：

- Phase A
- Phase B

目标：

- QukaAI 具备内部可用的 memory runtime

验收：

- knowledge 自动进入 memory
- internal chat / external agent 都能 recall / hydrate

#### Milestone 2: Human-in-the-loop Memory

范围：

- Phase C
- Phase D

目标：

- 用户可视化管理 memory
- 系统可做持续 consolidation

验收：

- 用户能查看和纠正记忆
- reflect 可以稳定产出 episodic / semantic

#### Milestone 3: OpenClaw Replacement Layer

范围：

- Phase E

目标：

- 形成 OpenClaw-facing skill contract

验收：

- OpenClaw-style runtime 可通过 skill adapter 接入
- Markdown projection 可稳定生成

### 14.9 人力与复杂度评估

按一名后端工程师主导估算：

- Phase A: 3-5 个工作日
- Phase B: 5-8 个工作日
- Phase C: 5-8 个工作日
- Phase D: 4-6 个工作日
- Phase E: 4-7 个工作日

粗略总量：

- 仅后端最小 runtime 闭环：约 2 周
- 加上 UI 和治理闭环：约 3-4 周
- 加上 OpenClaw skill adapter：约 4-5 周

如果前后端并行，可压缩总周期；如果单人串行开发，则应按完整 4-5 周估计。

### 14.10 当前最推荐的起手顺序

如果只做最小正确路径，建议从以下顺序开始：

1. 先落 migration + types + store
2. 再落 `remember / recall / hydrate / pin`
3. 再把 knowledge 创建流程接到 `remember`
4. 再补 knowledge-to-memory 的查询展示
5. 最后再做 reflect 和 OpenClaw skill adapter

这样做的原因是：

- 没有 stable schema，后续逻辑都会返工
- 没有 runtime API，外部 agent 无法接入
- 没有 UI 映射，人无法验证系统到底“记住了什么”
- 没有 skill adapter，暂时还不影响 QukaAI 自己先完成 memory runtime

## 15. 结论

这是一套面向 OpenClaw 记忆模型的最小适配方案。其核心思想是：

- 保留现有 knowledge 体系作为正文与检索底座
- 用 `quka_memory` 补出分层记忆语义
- 用 `quka_memory_edge` 补出证据链与反思关系
- 用 `remember / recall / reflect / hydrate` 补齐 agent runtime 的关键动作
- 用 knowledge UI 把记忆系统暴露给人，并允许人直接参与创建和修正

这样可以用最小改动把 QukaAI 从“知识库 + RAG”推进到“面向 agent 的共享记忆层”，同时满足 agent 自主记忆和 human-in-the-loop 管理两个目标。
