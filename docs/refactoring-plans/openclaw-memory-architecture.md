# OpenClaw x QukaAI Memory 架构图

**关联计划**: `openclaw-memory-minimal-adaptation`  
**日期**: 2026-04-02  
**状态**: 草案  

## 1. 目标

本文档用于说明在当前 memory 设计方案下，OpenClaw 如何接入 QukaAI，并通过 QukaAI 提供的 `knowledge + memory` 系统完成记忆的存储、召回、反思与上下文装配。

当前推荐接入路径为：

- `OpenClaw adapter skill + QukaAI HTTP API`

MCP tool 可以作为后续可选封装，但不是首选集成路径。

重点回答三个问题：

1. OpenClaw 与 QukaAI 的职责边界是什么
2. OpenClaw 如何通过 agent skill adapter 使用 QukaAI 的 memory 能力
3. QukaAI 内部的 handler / logic / store / db 应如何协作

## 2. 高层架构图

实现后的整体形态如下：

```text
+--------------------------------------------------------------+
|                         OpenClaw Runtime                     |
|--------------------------------------------------------------|
| Agent Loop                                                   |
| - plan / act / observe                                       |
| - decide when to search memory                               |
| - decide when to retain / reflect                            |
|                                                              |
| Memory Tool Surface                                          |
| - memory_search                                              |
| - memory_get                                                 |
| - memory_flush / reflect                                     |
+---------------------------|----------------------------------+
                            |
                            | OpenClaw adapter skill + HTTP API contract
                            v
+--------------------------------------------------------------+
|                  OpenClaw Adapter Skill                      |
|--------------------------------------------------------------|
| Skill / API Adapter                                          |
| - memory_search -> QukaAI /memory/recall                     |
| - memory_get    -> QukaAI /memory/get                        |
| - memory_flush  -> QukaAI /memory/remember or /reflect       |
|                                                              |
| Projection Adapter                                           |
| - MEMORY.md view                                             |
| - daily memory view                                          |
| - OpenClaw-friendly result formatting                        |
+---------------------------|----------------------------------+
                            |
                            v
+--------------------------------------------------------------+
|                     QukaAI Memory Runtime                    |
|--------------------------------------------------------------|
| Memory APIs                                                  |
| - remember                                                   |
| - recall                                                     |
| - hydrate                                                    |
| - reflect                                                    |
| - pin                                                        |
| - update / delete                                            |
|                                                              |
| Runtime Logic                                                |
| - lazy activation                                            |
| - progressive governance                                     |
| - dedupe / conflict mark                                     |
| - ranking / rerank hook                                      |
| - lifecycle: registered -> activated -> consolidated         |
+---------------------------|----------------------------------+
                            |
                            v
+--------------------------------------------------------------+
|                  QukaAI Knowledge + Memory Store             |
|--------------------------------------------------------------|
| Canonical Content Store                                      |
| - quka_knowledge                                             |
| - quka_knowledge_chunk                                       |
| - quka_vectors                                               |
|                                                              |
| Memory Catalog                                               |
| - quka_memory                                                |
| - quka_memory_edge                                           |
| - quka_memory_binding                                        |
|                                                              |
| Session / Chat Support                                       |
| - quka_chat_summary                                          |
| - quka_chat_session_pin                                      |
+--------------------------------------------------------------+
```

## 3. 职责分层

### 3.1 OpenClaw 负责什么

OpenClaw 继续负责 agent runtime 本身：

- 决定什么时候需要记忆
- 决定什么时候搜索记忆
- 决定什么时候把新的观察写入记忆
- 决定什么时候做 reflect / flush

也就是说，OpenClaw 负责“使用记忆”的决策。

### 3.2 QukaAI 负责什么

QukaAI 负责 memory backend 和 human-facing content layer：

- 存储 canonical content
- 存储 structured memory metadata
- 提供 `remember / recall / hydrate / reflect`
- 维护 memory lifecycle
- 提供 knowledge UI 给人查看
- 提供原生 memory API，供 agent 侧 skill 做 OpenClaw 语义映射

也就是说，QukaAI 负责“记忆真正落在哪里、如何检索、如何治理”。

### 3.3 为什么要有 Agent Skill Adapter

OpenClaw 原生偏向：

- `memory_search`
- `memory_get`
- Markdown workspace

而 QukaAI 内部偏向：

- `remember / recall / hydrate / reflect`
- structured DB-backed memory

因此需要 agent skill adapter 做语义转换，而不是让 OpenClaw 直接理解 QukaAI 内部结构，也不是在 QukaAI server 中新增 OpenClaw 专用兼容入口。

## 4. 关键调用链

### 4.1 搜索记忆

OpenClaw 需要相关记忆时，调用链如下：

```text
[OpenClaw Agent]
    |
    | 1. 当前任务需要补充记忆
    v
[memory_search]
    |
    v
[OpenClaw Adapter Skill]
    |
    | map to /memory/recall
    v
[QukaAI Memory Logic]
    |
    | recall from:
    | - quka_memory
    | - quka_knowledge_chunk
    | - quka_vectors
    v
[Top memories + canonical knowledge]
    |
    v
[Skill formats response]
    |
    v
[OpenClaw Agent gets memory snippets]
```

说明：

- OpenClaw 仍然认为自己在做 `memory_search`
- 实际底层已经变成 QukaAI 的 structured recall

### 4.2 读取某条记忆

当 OpenClaw 拿到某个 memory 引用并想读取详情时：

```text
[OpenClaw Agent]
    |
    | memory_get(memory_ref)
    v
[OpenClaw Adapter Skill]
    |
    | POST /memory/get with memory_id
    v
[QukaAI Memory API]
    |
    | access check + canonical content
    v
[full content]
    |
    v
[OpenClaw Agent]
```

说明：

- `memory_get` 不一定直接返回某张 DB 表记录
- 它通过 QukaAI `/memory/get` 返回 canonical content，并保留 memory layer / provenance metadata

### 4.3 写入记忆

当 OpenClaw 判定某个观察值得被记住时：

```text
[OpenClaw Agent]
    |
    | retain / flush important context
    v
[OpenClaw Adapter Skill]
    |
    | map to /memory/remember
    v
[QukaAI Memory Logic]
    |
    | create/update:
    | - quka_knowledge
    | - quka_memory
    | - optional edge/binding
    v
[knowledge + memory stored]
```

说明：

- OpenClaw 看到的是 retain/flush
- QukaAI 底层做的是 knowledge + memory 双层写入

### 4.4 反思沉淀

当 OpenClaw 或后台系统需要把碎片上下文沉淀成长期记忆时：

```text
[OpenClaw Agent or background job]
    |
    | reflect
    v
[QukaAI /memory/reflect]
    |
    | derive:
    | episodic -> semantic/core
    | add edges / evidence
    v
[quka_memory updated]
```

说明：

- 这一步对应 daily logs 到 curated memory 的演化
- 是从 `activated` 向 `consolidated` 推进的关键动作

## 5. QukaAI 内部实现组件图

从代码实现视角，可以把内部拆成以下层次：

```text
+---------------------------------------------------+
|                 HTTP / MCP / Skill Entry          |
|---------------------------------------------------|
| cmd/service/handler/memory.go                     |
| pkg/mcp/tools/...                                 |
| OpenClaw adapter skill -> native /memory/* API    |
+-------------------------|-------------------------+
                          v
+---------------------------------------------------+
|                  Application Logic Layer          |
|---------------------------------------------------|
| app/logic/v1/memory.go                            |
| - remember                                        |
| - recall                                          |
| - hydrate                                         |
| - reflect                                         |
| - pin                                             |
| - lifecycle promotion                             |
+-------------------------|-------------------------+
                          v
+---------------------------------------------------+
|                  Domain Storage Layer             |
|---------------------------------------------------|
| app/store/sqlstore/memory.go                      |
| app/store/sqlstore/memory_edge.go                 |
| app/store/sqlstore/memory_binding.go              |
| app/store/sqlstore/knowledge.go                   |
| app/store/sqlstore/knowledge_chunk.go             |
| app/store/sqlstore/chat_summary.go                |
| app/store/sqlstore/chat_session_pin.go            |
+-------------------------|-------------------------+
                          v
+---------------------------------------------------+
|                     PostgreSQL / Vector DB        |
|---------------------------------------------------|
| quka_knowledge                                    |
| quka_knowledge_chunk                              |
| quka_vectors                                      |
| quka_memory                                       |
| quka_memory_edge                                  |
| quka_memory_binding                               |
| quka_chat_summary                                 |
| quka_chat_session_pin                             |
+---------------------------------------------------+
```

## 6. Memory Lifecycle 推进图

当前方案中，memory lifecycle 由系统内部自动推进，而不是由用户手工控制。

```text
[Knowledge Promoted To Memory]
    |
    | remember / pin / reflect / governance
    v
[registered]
    |
    | recall / hydrate / pin hit by agent
    v
[activated]
    |
    | reflect / background consolidation
    v
[consolidated]
```

### 6.1 registered

含义：

- knowledge 已创建并被提升为 memory
- 对应 memory 已登记
- 还没被 agent 真正消费

触发者：

- `/memory/remember` 传入已有 `knowledge_id`
- agent 或用户显式 pin 某条 knowledge
- reflect 或后台治理流程创建 memory

### 6.2 activated

含义：

- 该 memory 已被 agent runtime 实际使用

触发者：

- `recall`
- `hydrate`
- `pin`

### 6.3 consolidated

含义：

- 该 memory 已参与更重的治理或沉淀

触发者：

- `reflect`
- 后台 consolidation job
- merge / supersede / evidence linking

## 7. 人与 Agent 的交互边界图

实现后的产品边界建议如下：

```text
                +----------------------+
                |        User          |
                +----------|-----------+
                           |
                           | read / edit
                           v
                +----------------------+
                |      Knowledge UI    |
                | - list / detail      |
                | - content edit       |
                | - memory badge       |
                +----------|-----------+
                           |
                           | canonical mapping
                           v
                +----------------------+
                |    QukaAI Memory     |
                | - runtime metadata   |
                | - lifecycle          |
                | - recall / reflect   |
                +----------|-----------+
                           ^
                           | read / write / govern
                           |
                +----------|-----------+
                |      OpenClaw        |
                |    Agent Runtime     |
                +----------------------+
```

说明：

- 用户主要面对 knowledge
- agent 主要面对 memory
- 两者通过 canonical mapping 连接
- 普通用户不直接操作 memory runtime

## 8. 最终理解

实现后，OpenClaw 使用 QukaAI 的方式可以概括为：

1. OpenClaw 继续做 agent runtime
2. OpenClaw 不直接操作 QukaAI 的底层表
3. OpenClaw 通过 agent skill adapter 调用 QukaAI 原生 memory API
4. QukaAI 负责将这些调用落到 `knowledge + memory` 双层系统
5. 用户通过 knowledge UI 间接看到 agent 的记忆结果

所以这不是“OpenClaw 接管 QukaAI 的 memory”，而是：

- OpenClaw 作为 memory client
- QukaAI 作为 memory platform

这也是当前方案最核心的架构定位。

## 9. 关键时序图

本节从实现视角说明 OpenClaw 通过 agent skill 调用原生 memory API 后，QukaAI 内部如何经过 `handler / logic / store / db` 完成 memory 操作。

### 9.1 `remember` 时序图

```text
[OpenClaw Agent]
    |
    | retain / flush important context
    v
[OpenClaw Adapter Skill]
    |
    | POST /memory/remember
    v
[Memory Handler]
    |
    v
[Memory Logic]
    |
    | 1. normalize input
    | 2. derive memory metadata
    | 3. optional dedupe / conflict mark
    | 4. ensure canonical knowledge
    v
[Knowledge Store] -----> [quka_knowledge]
    |
    v
[Memory Store] --------> [quka_memory]
    |
    v
[Response: memory_id + knowledge_id]
```

说明：

- 如果输入本身来自 knowledge 创建流程，`remember` 可以作为内部逻辑被自动触发
- knowledge 是 canonical content，memory 是 runtime metadata

### 9.2 `recall` 时序图

```text
[OpenClaw Agent]
    |
    | memory_search(query)
    v
[OpenClaw Adapter Skill]
    |
    | POST /memory/recall
    v
[Memory Handler]
    |
    v
[Memory Logic]
    |
    | 1. parse runtime_context
    | 2. build candidate set from quka_memory
    | 3. retrieve chunks / vectors
    | 4. rank by relevance + importance + confidence
    | 5. mark hit memories as activated
    v
[Memory Store] --------> [quka_memory]
[Chunk Store] ---------> [quka_knowledge_chunk]
[Vector Store] --------> [quka_vectors]
    |
    v
[Top memory hits + canonical content]
    |
    v
[Skill formats response]
    |
    v
[OpenClaw Agent]
```

说明：

- `recall` 是 OpenClaw `memory_search` 的核心映射
- recall 命中的 memory 会更新访问统计，并推进 lifecycle

### 9.3 `hydrate` 时序图

```text
[OpenClaw Agent]
    |
    | new runtime begins
    v
[OpenClaw Adapter Skill]
    |
    | POST /memory/hydrate
    v
[Memory Handler]
    |
    v
[Memory Logic]
    |
    | 1. resolve runtime_context
    | 2. fetch pinned/working memories
    | 3. fetch core + recent episodic + semantic
    | 4. assemble memory pack
    | 5. optionally cache hydration result
    v
[Memory Binding Store] -> [quka_memory_binding]
[Memory Store] --------> [quka_memory]
[ChatSessionPin Store] -> [quka_chat_session_pin] (chat_session only)
    |
    v
[assembled_context + memory ids]
    |
    v
[OpenClaw Agent]
```

说明：

- `hydrate` 更像预装配，而不是临时搜索
- 对 `chat_session`，可利用 `quka_chat_session_pin` 做缓存
- 对 `agent_run / task / workspace`，主要依赖 `quka_memory_binding`

### 9.4 `reflect` 时序图

```text
[OpenClaw Agent or Background Job]
    |
    | reflect
    v
[OpenClaw Adapter Skill / Scheduler]
    |
    | POST /memory/reflect
    v
[Memory Handler]
    |
    v
[Memory Logic]
    |
    | 1. read source context
    | 2. derive episodic memory
    | 3. promote to semantic/core if needed
    | 4. create edges
    | 5. mark memory as consolidated
    v
[ChatSummary Store] ---> [quka_chat_summary]
[Memory Store] --------> [quka_memory]
[MemoryEdge Store] ----> [quka_memory_edge]
    |
    v
[consolidated memories]
```

说明：

- `reflect` 可以由 agent 主动触发，也可以由后台队列异步运行
- 它是 lifecycle 从 `activated` 到 `consolidated` 的关键步骤

### 9.5 `pin` 时序图

```text
[OpenClaw Agent]
    |
    | pin memory to runtime_context
    v
[OpenClaw Adapter Skill]
    |
    | POST /memory/pin
    v
[Memory Handler]
    |
    v
[Memory Logic]
    |
    | 1. validate runtime_context
    | 2. create/update bindings
    | 3. mark pinned memory as activated
    | 4. sync chat cache if needed
    v
[MemoryBinding Store] --> [quka_memory_binding]
[ChatSessionPin Store] -> [quka_chat_session_pin] (chat_session only)
    |
    v
[binding created]
```

说明：

- `pin` 固定的是 working set
- 目标不是 session 本身，而是一般化的 `runtime_context`

### 9.6 `update / delete` 时序图

```text
[OpenClaw Agent or Internal Governance Job]
    |
    | update / delete memory
    v
[OpenClaw Adapter Skill or Internal API Caller]
    |
    | POST /memory/update or /memory/delete
    v
[Memory Handler]
    |
    v
[Memory Logic]
    |
    | update:
    | - revise canonical knowledge
    | - update memory metadata
    | - optionally create supersedes edge
    |
    | delete:
    | - soft delete or hard delete memory
    | - exclude from future recall / hydrate
    v
[Knowledge Store] -----> [quka_knowledge]
[Memory Store] --------> [quka_memory]
[MemoryEdge Store] ----> [quka_memory_edge] (optional)
```

说明：

- `update / delete` 更偏治理能力
- 它们不一定是 OpenClaw 高频使用动作，但对长期 memory hygiene 很重要

## 10. API 到实现层映射表

```text
OpenClaw action           Skill mapping                QukaAI core path
---------------------------------------------------------------------------
memory_search             POST /memory/recall         handler -> logic -> memory/chunk/vector store
memory_get                POST /memory/get            handler -> logic -> memory access + knowledge store
retain / flush            POST /memory/remember       handler -> logic -> knowledge + memory store
startup memory load       POST /memory/hydrate        handler -> logic -> binding + memory store
reflect / consolidate     POST /memory/reflect        handler -> logic -> summary + memory + edge store
pin working memories      POST /memory/pin            handler -> logic -> binding store
govern memory             POST /memory/update/delete  handler -> logic -> memory store
```

## 11. 实现重点

从工程角度，最关键的不是单独做每个 API，而是保证以下三件事一致：

1. `knowledge` 与 `memory` 的 canonical mapping 一致
2. `runtime_context` 与 `quka_memory_binding` 的绑定语义一致
3. lifecycle 推进规则在 `remember / recall / hydrate / reflect / pin` 之间一致

如果这三点成立，OpenClaw 接入 QukaAI 后就会表现为：

- OpenClaw 继续按自己的习惯使用 memory
- QukaAI 在内部以结构化方式完成真正的存储和治理
