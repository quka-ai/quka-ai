# QukaAI 作为 Hermes 原生 Memory Provider 的能力报告

**日期**: 2026-06-03  
**状态**: 已完成  
**作者**: Codex  
**适用范围**: QukaAI memory 系统、Hermes Agent memory provider 插件、Hermes 原生 memory provider 管理面

## 1. 结论

当前 QukaAI memory 系统已经可以作为 Hermes Agent 的外部 memory provider 接入 Hermes 原生 memory 管理流程。

这里的“原生”不是指替换或删除 Hermes 内置 `MEMORY.md / USER.md` 文件记忆，而是指 QukaAI 通过 Hermes 官方 `MemoryProvider` 抽象接入 Hermes agent loop，由 Hermes 原生的 provider 加载、配置、工具注入、prefetch、session hook、built-in memory write mirror 等机制统一调度。

换句话说：

- Hermes 仍保留自己的 built-in memory。
- QukaAI 作为一个 Hermes external memory provider 被选择和加载。
- Hermes 用原生 MemoryProvider 生命周期调用 QukaAI 插件。
- QukaAI 提供实际的持久化、召回、hydrate、pin、reflect、delete/forget、runtime context 绑定和跨 session memory backend 能力。

这符合 Hermes 官方 memory provider 模型：外部 provider 是 additive 的，内置 memory 仍然工作；同一时间只启用一个外部 provider。

## 2. Hermes 原生 Memory Provider 管理需要什么

Hermes 的 memory provider 管理面核心由 `agent/memory_provider.py`、`agent/memory_manager.py` 和 `plugins/memory/__init__.py` 组成。

一个可部署的 Hermes memory provider 需要满足四类要求。

### 2.1 插件加载与注册

Hermes 通过 `$HERMES_HOME/plugins/<name>/` 或内置 `plugins/memory/<name>/` 发现 memory provider。插件需要：

- 提供 `__init__.py`。
- 实现 `MemoryProvider` 子类。
- 提供 `register(ctx)`，并调用 `ctx.register_memory_provider(...)`。
- 可被配置为：

```yaml
memory:
  provider: qukaai
```

QukaAI 已实现：

- `integrations/hermes-memory-provider/qukaai/__init__.py`
- `integrations/hermes-memory-provider/qukaai/plugin.yaml`
- `register(ctx)` 注册入口
- `QukaAIMemoryProvider(MemoryProvider)`

### 2.2 生命周期契约

Hermes 会通过 MemoryProvider 生命周期调度外部 provider：

- `is_available()`
- `initialize(session_id, **kwargs)`
- `system_prompt_block()`
- `queue_prefetch(query, session_id="")`
- `prefetch(query, session_id="")`
- `sync_turn(user_content, assistant_content, session_id="", messages=None)`
- `get_tool_schemas()`
- `handle_tool_call(tool_name, args, **kwargs)`
- `on_session_end(messages)`
- `on_pre_compress(messages)`
- `on_session_switch(...)`
- `on_memory_write(action, target, content, metadata=None)`
- `shutdown()`

QukaAI provider 已实现这些核心方法，并把它们映射到 QukaAI memory HTTP API。

### 2.3 工具注入

Hermes 会把 provider 的工具 schema 注入模型工具面。QukaAI 暴露：

- `qukaai_memory_search`
- `qukaai_memory_remember`
- `qukaai_memory_forget`
- `qukaai_memory_hydrate`
- `qukaai_memory_pin`

这些工具分别映射到：

- `/memory/recall`
- `/memory/remember`
- `/memory/delete`
- `/memory/hydrate`
- `/memory/pin`

### 2.4 内置 memory 写入镜像

Hermes 会在 built-in memory 写入时通过 `on_memory_write` 通知外部 provider。QukaAI provider 已支持镜像：

- `add / replace` -> `/memory/remember`
- `remove` -> recall 后匹配删除
- `target=user` 默认进入 `user_global`
- `target=memory` 默认进入配置的 memory layer

这意味着用户继续使用 Hermes 原生命令或 built-in memory 工具时，QukaAI 可以同步持久化对应的 agent memory。

## 3. QukaAI 为什么能承接 Hermes Memory Backend

Hermes 的 MemoryProvider 抽象本质上需要一个 backend 提供以下能力：

| Hermes 管理能力 | QukaAI 对应能力 | 当前状态 |
| --- | --- | --- |
| 启动初始化和配置检查 | `QukaAIMemoryProvider.is_available / initialize` | 已实现 |
| pre-turn context recall | `queue_prefetch -> /memory/hydrate`，`prefetch` 返回缓存 context | 已实现 |
| 模型主动记忆工具 | `qukaai_memory_remember -> /memory/remember` | 已实现 |
| 模型主动搜索工具 | `qukaai_memory_search -> /memory/recall` | 已实现 |
| 模型主动遗忘工具 | `qukaai_memory_forget -> /memory/delete` | 已实现 |
| runtime pinned memory | `qukaai_memory_pin -> /memory/pin` + `quka_memory_binding` | 已实现 |
| session end extraction | `on_session_end -> /memory/reflect` | 已实现 |
| context compression 前沉淀 | `on_pre_compress -> /memory/reflect` | 已实现 |
| session 切换 | `on_session_switch` 更新 runtime context | 已实现 |
| built-in memory mirror | `on_memory_write` | 已实现 |
| provider shutdown | join 后台线程 | 已实现 |

QukaAI 后端的核心优势是：它不是一个简单 key-value 记忆文件，而是一套 agent memory runtime backend。

## 4. Memory 与 Knowledge 的边界已经满足 Hermes 场景

Hermes memory provider 需要的是 agent memory，不是用户资料库。当前 QukaAI 已明确分层：

- `Knowledge`: 用户知识库，面向文档、笔记、引用资料、RAG 内容管理。
- `Memory`: agent runtime memory，面向偏好、约定、长期决策、短期 working context、session reflection。
- Durable memory backing knowledge: durable memory 的内部内容背板，使用 `resource="__memory__"`，不会出现在普通 knowledge list/detail/update/delete 路径。
- Working memory inline content: 短生命周期 memory 直接使用 `quka_memory.content` 加密保存，不创建 knowledge，不进入摘要、分块和向量化流水线。

这解决了 Hermes provider 最容易出错的一个边界：不能把用户知识库里的每条资料都当成 agent memory，也不能让 agent runtime memory 污染用户知识库。

## 5. QukaAI Memory Backend 的内部能力

### 5.1 持久记忆目录

`quka_memory` 保存：

- memory type: `core / semantic / episodic / working`
- scope/layer: `user_global / user_space / space_shared`
- status: `active / archived / superseded / deleted`
- confidence / importance
- author type / epistemic status
- source kind / source ref
- entity key / dedupe key / conflict state
- last accessed / access count

这些字段让 Hermes 不只是“写几行 memory text”，而是获得可治理、可排序、可分层、可审计的 agent memory。

### 5.2 Durable Memory

Durable memory 继续使用 hidden backing knowledge：

- 复用 QukaAI 加密能力。
- 复用 knowledge chunk。
- 复用 vector embedding。
- 复用异步处理流水线。
- 可通过 memory API 管理生命周期。
- 不暴露到用户普通 knowledge 列表。

适用内容：

- 用户长期偏好。
- 项目长期约定。
- 反复确认的事实。
- session reflection 后的 episodic memory。
- 需要跨 session 召回的 semantic/core memory。

### 5.3 Working Memory

Working memory 改为 inline content：

- 不创建 knowledge。
- 不触发摘要、分块、embedding。
- 直接加密存入 `quka_memory.content`。
- 可被 `get / recall / hydrate / pin / update / delete` 使用。
- 适合作为 Hermes 当前 session / agent run 的 runtime working set。

这与 Hermes 的 runtime context 语义更匹配：working memory 是短期执行上下文，不应默认变成用户知识库内容。

### 5.4 Runtime Context

QukaAI 支持通用 runtime context：

- `chat_session`
- `agent_run`
- `task`
- `workspace`

Hermes provider 使用：

```text
hermes:<agent_identity>:<platform>:<session_id>
```

作为 `agent_run` context id。这样 Hermes 的 session/resume/branch/compression 事件可以被映射到 QukaAI 的 runtime context，不依赖 QukaAI 内部 chat session。

### 5.5 Recall 与 Hydrate

QukaAI memory recall 不是全量搜索 knowledge，而是：

1. 先按当前用户可访问 memory layer 过滤 active memory。
2. durable memory 用 backing knowledge 的 vector/chunk 能力召回。
3. inline working memory 用 title、entity_key、解密正文做轻量召回。
4. 按 importance、confidence、updated_at 排序。
5. 返回结构化 memory 元数据和正文。

Hydrate 会把 pinned/working memories 与 recall memories 合并，按 token budget 组装为可注入模型上下文的文本。

这正好对应 Hermes `prefetch` 的需求：在模型调用前，把相关长期记忆和当前 working context 作为 memory context 注入。

## 6. 部署形态

### 6.1 QukaAI 侧

部署 QukaAI 后端，确保：

- PostgreSQL + pgvector 可用。
- QukaAI 服务可被 Hermes 访问。
- 已创建 Hermes 使用的 QukaAI space。
- 已提供可访问该 space 的 token。

### 6.2 Hermes 侧

将插件目录复制或 symlink 到 Hermes profile：

```bash
mkdir -p "$HERMES_HOME/plugins"
ln -s /path/to/quka-ai/integrations/hermes-memory-provider/qukaai "$HERMES_HOME/plugins/qukaai"
```

配置 Hermes：

```yaml
memory:
  provider: qukaai
```

配置环境变量：

```bash
export QUKA_API_BASE_URL="http://localhost:8080/api/v1"
export QUKA_SPACE_ID="your_space_id"
export QUKA_ACCESS_TOKEN="your_access_token"
```

也可以使用 `$HERMES_HOME/qukaai-memory.json` 保存非 secret 配置。

## 7. 为什么它能提供 Hermes 原生 Memory 管理体验

### 7.1 Hermes 仍然负责调度

QukaAI 没有绕过 Hermes agent loop。Hermes 仍然负责：

- provider discovery
- provider selection
- provider initialization
- system prompt assembly
- prefetch injection
- tool schema injection
- tool call routing
- session switch notification
- session end notification
- built-in memory write mirror
- provider shutdown

QukaAI 只是实现这个原生接口背后的 memory backend。

### 7.2 QukaAI 提供比文件记忆更完整的 backend

Hermes built-in memory 更像 profile 文件记忆。QukaAI 提供：

- 数据库持久化。
- 多 layer 隔离。
- space 访问控制。
- runtime context binding。
- vector recall。
- inline working memory。
- durable hidden backing knowledge。
- memory edges。
- confidence / importance / epistemic status。
- delete/forget 生命周期。

因此它可以承担 Hermes 外部 memory provider 的主 backend，而不只是把内容同步到一个远端文本文件。

### 7.3 Memory 与 Knowledge 的边界适合 agent 场景

QukaAI 没有把用户 knowledge 全部暴露为 memory。Hermes agent 看到的是 agent memory 语义：

- 什么时候该记住。
- 什么时候该搜索记忆。
- 什么时候该 pin 到 runtime context。
- 什么时候该 forget。

这避免了 RAG 知识库和 agent memory 混用导致的上下文污染。

## 8. 当前限制

当前实现已经满足 Hermes provider 部署与管理的主路径，但仍有可增强项：

- Reflect 当前主要沉淀 summary/messages，后续可以引入更强 LLM extraction、去噪、合并。
- Recall 当前有基础排序，后续可以加入 reranker。
- Conflict/dedupe 字段已经存在，但冲突治理策略还可以继续增强。
- Working memory 目前不做 vector index，这是设计选择；若未来 working memory 变长或跨 context 使用，可再评估轻量索引。

## 9. 证据文件

QukaAI 侧：

- `integrations/hermes-memory-provider/qukaai/__init__.py`
- `integrations/hermes-memory-provider/qukaai/plugin.yaml`
- `integrations/hermes-memory-provider/qukaai/README.md`
- `app/logic/v1/memory.go`
- `cmd/service/handler/memory.go`
- `app/store/sqlstore/memory.sql`
- `app/store/sqlstore/migrations/working_memory_inline_content.sql`
- `docs/architecture/memory-knowledge-relationship.md`

Hermes 侧参考：

- https://github.com/NousResearch/hermes-agent/blob/main/agent/memory_provider.py
- https://github.com/NousResearch/hermes-agent/blob/main/agent/memory_manager.py
- https://github.com/NousResearch/hermes-agent/blob/main/plugins/memory/__init__.py
- https://github.com/NousResearch/hermes-agent/blob/main/website/docs/developer-guide/memory-provider-plugin.md
- https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/memory-providers.md

## 10. 总结

QukaAI 当前 memory 系统已经满足作为 Hermes external memory provider 的部署条件：插件层符合 Hermes 原生 MemoryProvider 契约，后端层提供完整 agent memory 管理能力，产品语义上严格区分 agent memory 与用户 knowledge。

因此，QukaAI 可以作为 Hermes 原生 memory provider 管理面下的外部 memory backend，为 Hermes 提供跨 session、可治理、可召回、可 hydrate、可 pin、可 reflect、可 forget 的 memory 管理能力。
