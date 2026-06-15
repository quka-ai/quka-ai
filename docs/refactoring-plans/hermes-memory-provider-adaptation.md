# Hermes Agent Memory Provider 适配调研与改造计划

**计划ID**: hermes-memory-provider-adaptation  
**日期**: 2026-06-03  
**状态**: 阶段一已实施  
**优先级**: 高  
**作者**: Codex  

## 1. 背景

本次调研目标是判断 QukaAI 当前 memory 能力是否可以作为 Hermes Agent 的 memory provider，并明确不满足时的改造方案。

这里需要先区分两层含义：

- **memory backend 能力**: QukaAI 后端是否具备 agent memory 所需的存储、检索、上下文装配、生命周期和治理能力。
- **Hermes provider 插件能力**: Hermes 是否能直接加载一个 QukaAI provider，并在 agent loop 中调用其生命周期方法。

当前结论是：QukaAI 后端已基本具备 Hermes memory provider 背后的 memory backend 能力，但原先还不是 Hermes 可直接加载的 memory provider。现已新增一个 Python 插件适配层，把 Hermes 的 `MemoryProvider` 生命周期映射到 QukaAI 的 HTTP memory API。

## 实施记录

2026-06-03 已完成阶段一最小可用适配层：

- 新增 `integrations/hermes-memory-provider/qukaai/__init__.py`。
- 新增 `integrations/hermes-memory-provider/qukaai/plugin.yaml`。
- 新增 `integrations/hermes-memory-provider/qukaai/README.md`。
- 新增 `integrations/hermes-memory-provider/qukaai/tests/test_qukaai_provider.py`。
- 已验证插件可在 Hermes `MemoryProvider` 基类环境下导入并通过 `register(ctx)` 注册。
- 已验证 Python 单元测试和 QukaAI memory 相关 Go 窄测试。

后续 working memory 改造已调整 Go 后端 schema。QukaAI 仍保持 `memory` 与 `knowledge` 分层：普通 knowledge 不会自动成为 agent memory，durable memory API 创建的背板内容仍使用隐藏 `__memory__` resource，working memory 可使用 `quka_memory` 内联加密内容而不创建 backing knowledge。

## 2. Hermes Agent Memory Provider 需要具备的能力

Hermes 官方抽象位于 `agent/memory_provider.py`。Memory provider 是一个单选外部插件，内置 memory 始终存在，外部 provider 同时只允许启用一个。

### 2.1 必须实现的插件契约

Hermes provider 至少要实现：

- `name`: provider 短名称，例如 `qukaai`。
- `is_available()`: 判断配置、依赖和凭证是否就绪；不得做网络调用。
- `initialize(session_id, **kwargs)`: agent 启动时初始化，接收 `hermes_home`、`platform`、`agent_context`、`agent_identity`、`agent_workspace`、`user_id` 等上下文。
- `get_tool_schemas()`: 返回暴露给模型的 OpenAI function calling 工具 schema。
- `handle_tool_call(tool_name, args, **kwargs)`: 处理 provider 工具调用并返回 JSON 字符串。
- `get_config_schema()`: 给 `hermes memory setup` 声明配置项。
- `save_config(values, hermes_home)`: 保存非 secret 配置；如果全部使用环境变量则可保留 no-op。

### 2.2 核心生命周期能力

Hermes agent loop 会调用：

- `system_prompt_block()`: 注入静态 memory provider 说明。
- `prefetch(query, session_id="")`: 在模型调用前返回已召回的上下文文本。
- `queue_prefetch(query, session_id="")`: 当前 turn 完成后预热下一轮召回。
- `sync_turn(user_content, assistant_content, session_id="", messages=None)`: 当前 turn 完成后非阻塞写入或沉淀记忆。
- `shutdown()`: 退出时清理后台线程、队列和连接。

### 2.3 可选 hook

高质量 provider 通常还应实现：

- `on_turn_start(turn_number, message, **kwargs)`: turn 计数、上下文维护、周期性治理。
- `on_session_end(messages)`: 会话结束时抽取长期记忆。
- `on_session_switch(new_session_id, parent_session_id="", reset=False, rewound=False, **kwargs)`: session 切换、resume、branch、context compression 后更新内部 session 状态。
- `on_pre_compress(messages) -> str`: context compression 前抽取会被丢弃的内容，并返回给 compressor 保留。
- `on_memory_write(action, target, content, metadata=None)`: 镜像 Hermes 内置 memory 工具写入。
- `on_delegation(task, result, child_session_id="", **kwargs)`: 父 agent 观察 subagent 结果。

### 2.4 插件安装与发现方式

Hermes 官方文档和 loader 约定：

- 插件目录形态为 `plugins/memory/<name>/`。
- 用户安装插件位于 `$HERMES_HOME/plugins/<name>/`。
- 每个插件至少包含 `__init__.py`，通常还包含 `plugin.yaml` 和 `README.md`。
- `__init__.py` 中需要实现 `MemoryProvider` 子类，并通过 `register(ctx)` 调用 `ctx.register_memory_provider(...)`。
- Hermes 主仓库已不再接收新的 memory provider；新的 backend 应作为独立插件仓库、用户插件目录或 pip entry point 分发。

## 3. QukaAI 当前 memory 能力评估

### 3.1 已满足的 backend 能力

当前 QukaAI 已经具备以下能力：

| Hermes provider 需要 | QukaAI 当前实现 | 评估 |
| --- | --- | --- |
| 持久 memory 存储 | `quka_memory` + backing `quka_knowledge` | 满足 |
| 语义召回 | `MemoryLogic.Recall` 限定 memory 候选后复用 `quka_vectors` | 基本满足 |
| 上下文装配 | `MemoryLogic.Hydrate` 返回 `assembled_context` 和结构化 `items` | 满足 |
| 工具式写入 | `RememberUserMemory` / `/memory/remember` | 满足 |
| 工具式查询 | `SearchUserMemories` / `/memory/recall` | 满足 |
| 遗忘 | `ForgetUserMemory` / `/memory/delete` with `delete_knowledge` | 满足 |
| session / agent run 绑定 | `quka_memory_binding` + `RuntimeContext` | 满足 |
| 会话结束沉淀 | `/memory/reflect` 支持 `agent_run/task/workspace` 的 `RuntimeContext.Extraction` | 基本满足 |
| memory 与 knowledge 区分 | durable memory 使用 `resource="__memory__"` 隐藏背板 knowledge；working memory 可内联内容；普通 knowledge 默认不注册 memory | 满足 |
| 访问边界 | `user_global`、`user_space`、`space_shared` 三层 | 满足 |
| 可信度、重要度、来源、实体键 | `confidence`、`importance`、`source_kind`、`source_ref`、`entity_key` | 满足 |
| 关系和证据链 | `quka_memory_edge` | 基本满足 |

### 3.2 memory 与 knowledge 的边界

当前实现已经正确区分：

- `Knowledge` 是用户知识库，承载内容正文、资源分组、分块、向量化和 RAG 检索。
- `Memory` 是 agent runtime 记忆目录与治理层，指向 `knowledge_id`，但不把所有 knowledge 都视作 memory。
- 普通 knowledge 创建后不会自动进入 memory。
- durable memory API 创建的背板 knowledge 使用 `__memory__` 资源，并被普通 knowledge 列表、详情、更新、删除路径隐藏或保护。
- working memory 可不创建 backing knowledge，而是使用 `quka_memory.content` 保存加密的短生命周期上下文。

这符合本次需求中“memory 是 ai-agent 的 memory，knowledge 是用户的知识库”的边界要求。

### 3.3 不满足 Hermes provider 的部分

原先不满足的部分不是核心后端能力，而是 Hermes 适配与运行契约。阶段一已补齐最小可用插件能力，仍可继续增强内置 memory 镜像、turn-by-turn 沉淀策略和后端反思质量。

1. **没有 Hermes Python provider 插件**（已补齐）
   - Hermes 无法直接加载 Go 后端逻辑。
   - 需要一个 `MemoryProvider` 子类作为桥接层。

2. **没有 Hermes 配置 schema**（已补齐）
   - 需要声明 `QUKA_API_BASE_URL`、`QUKA_ACCESS_TOKEN`、`QUKA_SPACE_ID`、默认 layer、token budget 等配置。

3. **没有 provider 工具 schema**（已补齐）
   - 需要把 QukaAI 的 `remember/recall/hydrate/forget/reflect/pin` 暴露成 Hermes provider tools。

4. **没有后台 prefetch/sync 线程**（已补齐最小版本）
   - Hermes 要求 `sync_turn()` 非阻塞。
   - `prefetch()` 应尽量快，通常消费 `queue_prefetch()` 的结果。

5. **Hermes session identity 到 QukaAI runtime context 的映射未定义**（已补齐）
   - 需要把 Hermes `session_id` 映射为 `RuntimeContext{type:"agent_run", id:"hermes:<session_id>"}` 或类似稳定 ID。
   - `agent_identity`、`agent_workspace`、`platform`、`user_id` 应进入 `source_ref` 或 extraction metadata。

6. **内置 memory 写入镜像未定义**（已补齐最小版本）
   - Hermes 的 `on_memory_write()` 可以把 `MEMORY.md/USER.md` 写入同步到 QukaAI。
   - 需要决定写入 `user_global` 还是 `user_space`，以及如何处理 replace/remove。

7. **错误、限流和鉴权体验需要适配**（已补齐最小版本）
   - QukaAI HTTP API 使用 `X-Access-Token` 或 `X-Authorization`。
   - provider 需要把错误整理成 Hermes tool result JSON，避免异常打断 agent loop。

## 4. 建议改造方案

### 4.1 总体原则

- 不改动当前 `knowledge` 作为内容底座的设计。
- 不让普通 knowledge 自动进入 Hermes memory。
- 将 QukaAI 作为外部 memory backend，Hermes 插件只做适配、格式化、异步队列和配置管理。
- 优先使用已有 `/memory/*` API，不新增后端表结构。
- 如果后续需要更强沉淀质量，再增加专门的 `memory/ingest-turn` 或后台 LLM extraction 能力。

### 4.2 新增 Hermes provider 插件目录

建议在本仓库新增：

```text
integrations/hermes-memory-provider/qukaai/
├── __init__.py
├── plugin.yaml
├── README.md
└── tests/
    └── test_qukaai_provider.py
```

发布或本地使用时，将 `integrations/hermes-memory-provider/qukaai/` 复制或 symlink 到：

```text
$HERMES_HOME/plugins/qukaai/
```

Hermes 配置中启用：

```yaml
memory:
  provider: qukaai
```

### 4.3 Provider 配置

建议环境变量：

- `QUKA_API_BASE_URL`: QukaAI 服务地址，例如 `http://localhost:8080/api/v1`。
- `QUKA_ACCESS_TOKEN`: QukaAI access token，对应 `X-Access-Token`。
- `QUKA_AUTH_TOKEN`: 可选，QukaAI auth token，对应 `X-Authorization`。
- `QUKA_SPACE_ID`: 默认 space id。

建议非 secret 配置文件 `$HERMES_HOME/qukaai-memory.json`：

```json
{
  "space_id": "",
  "default_layer": "user_space",
  "prefetch_limit": 6,
  "hydrate_token_budget": 1200,
  "sync_turn": "off",
  "reflect_on_session_end": true,
  "mirror_builtin_memory": true
}
```

### 4.4 Hermes 生命周期映射

| Hermes 方法 | QukaAI 映射 |
| --- | --- |
| `initialize(session_id, **kwargs)` | 保存配置和当前 session；构造 runtime context |
| `system_prompt_block()` | 返回 QukaAI memory 使用说明，强调 memory/knowledge 区分 |
| `queue_prefetch(query, session_id)` | 后台调用 `/memory/hydrate` 或 `/memory/recall` |
| `prefetch(query, session_id)` | 返回上一次 `queue_prefetch` 缓存的 `assembled_context` |
| `sync_turn(user, assistant, messages)` | 可选：后台调用 `/memory/reflect` 或先 no-op |
| `on_session_end(messages)` | 调用 `/memory/reflect`，传 `RuntimeContext.Extraction.Messages` |
| `on_pre_compress(messages)` | 调用 `/memory/reflect` 或本地格式化摘要，返回压缩提示补充 |
| `on_session_switch(new_session_id, ...)` | 更新 runtime context id，清空 prefetch cache |
| `on_memory_write(action, target, content, metadata)` | `add/replace` 映射 `/memory/remember`；`remove` 需先 recall 再 delete |
| `shutdown()` | join 后台线程并清理缓存 |

推荐 runtime context：

```json
{
  "type": "agent_run",
  "id": "hermes:<agent_identity>:<platform>:<session_id>"
}
```

### 4.5 Hermes 工具设计

建议暴露以下工具：

- `qukaai_memory_search`
  - 入参：`query`、`limit`、`memory_types`、`layer`
  - 映射：`POST /:spaceid/memory/recall`

- `qukaai_memory_remember`
  - 入参：`content`、`title`、`layer`、`memory_type`、`entity_key`、`importance`
  - 映射：`POST /:spaceid/memory/remember`
  - 默认：`author_type=agent`、`source_kind=chat`、`source_ref=hermes:<session_id>`

- `qukaai_memory_forget`
  - 入参：`memory_id`、`delete_knowledge`
  - 映射：`POST /:spaceid/memory/delete`

- `qukaai_memory_hydrate`
  - 入参：`query`、`token_budget`
  - 映射：`POST /:spaceid/memory/hydrate`

- `qukaai_memory_pin`
  - 入参：`memory_ids`、`binding_type`
  - 映射：`POST /:spaceid/memory/pin`

### 4.6 后端是否需要改造

当前不建议立即改动 Go 后端 schema。理由：

- `RuntimeContext.Extraction` 已能承载 Hermes messages。
- `/memory/reflect` 已支持非 QukaAI chat session 的 `agent_run/task/workspace`。
- `/memory/hydrate` 已支持不传 runtime context 的普通召回式 hydration。
- durable memory 背板 knowledge 已使用 `__memory__` 隐藏，working memory 可内联加密内容，边界符合要求。

可选增强项：

1. 增加 `source_kind = "hermes"` 或保持使用 `chat/import` 并在 `source_ref` 写入 `hermes:<session_id>`。
2. 增加 `/memory/ingest-turn`，用于显式接收 user/assistant turn 并由后端决定是否沉淀；当前可以先由 provider 控制 `sync_turn=off` 或只在 `on_session_end` reflect。
3. 增加 memory API 文档，说明第三方 agent provider 的 header、payload 和错误语义。
4. 改善 `Reflect` 的 LLM 抽取质量：当前传入 messages 时主要是拼接内容，后续可加入专门 prompt 抽取长期事实。

## 5. 实施步骤

### 阶段一：适配层最小可用

1. 新增 `integrations/hermes-memory-provider/qukaai/`。
2. 实现 `QukaAIMemoryProvider`。
3. 支持配置读取、availability 检查、HTTP client、错误 JSON 化。
4. 实现 `queue_prefetch/prefetch` 映射 `/memory/hydrate`。
5. 实现 `qukaai_memory_search`、`qukaai_memory_remember`、`qukaai_memory_forget`。
6. 实现 `on_session_switch` 和 `shutdown`。
7. 增加 README，说明安装到 `$HERMES_HOME/plugins/qukaai` 的方式。

### 阶段二：会话沉淀与压缩前抽取

1. 实现 `on_session_end(messages)` 调用 `/memory/reflect`。
2. 实现 `on_pre_compress(messages)`，在压缩前把即将丢弃的消息沉淀为 episodic memory。
3. 增加 `sync_turn` 配置，默认 `off`，避免每 turn 都写入噪声。
4. 可选实现后台队列，保证所有网络调用非阻塞。

### 阶段三：内置 memory 镜像与治理增强

1. 实现 `on_memory_write`。
2. 对 `add/replace/remove` 建立 QukaAI memory 的搜索、更新、删除策略。
3. 可选补充后端 `source_kind` 或 `ingest-turn` 接口。
4. 增加更强的重复、冲突、supersede 策略。

## 6. 关键考虑点

- Hermes 外部 memory provider 同时只能启用一个；QukaAI provider 不应试图和其他外部 provider 并行。
- `sync_turn()` 必须非阻塞，所有 HTTP 写入都要放到后台线程或禁用。
- provider 返回的 prefetch 文本不应自行包裹 `<memory-context>`，Hermes `MemoryManager` 会统一加 fence 并做 scrub。
- QukaAI memory 不应污染普通 knowledge 列表；durable memory 必须继续使用 `__memory__` 背板，working memory 不创建普通 knowledge。
- `space_shared` 应谨慎暴露给 agent 工具，默认仍使用 `user_space`。
- Hermes 的 `user_id`、`agent_identity`、`agent_workspace` 不等于 QukaAI 用户 ID；鉴权仍由 QukaAI token 决定，Hermes metadata 只作为 runtime/source 信息。
- access token 权限需要覆盖当前 space 的 view/edit，否则 search/remember 会失败。

## 7. 需要确认的问题

1. QukaAI provider 是否作为本仓库 `integrations/` 下的可复制插件交付，还是独立仓库/pip 包交付？
2. Hermes 自动 `sync_turn` 默认是否关闭？建议关闭，只在工具调用、session end、pre-compress 时写入。
3. 默认 layer 是否使用 `user_space`？建议是，`user_global` 只由模型工具显式选择。
4. 是否允许 provider 使用 `space_shared`？建议默认禁用，配置项打开。
5. 是否需要新增 `source_kind = "hermes"`？建议先不新增，使用 `source_kind=chat/import` + `source_ref=hermes:<session_id>`。

## 8. 相关文件

- `app/logic/v1/memory.go`
- `cmd/service/handler/memory.go`
- `pkg/types/memory.go`
- `pkg/ai/agents/memory/function.go`
- `app/logic/v1/memory_prompt.go`
- `app/store/sqlstore/memory.sql`
- `app/store/sqlstore/memory_binding.sql`
- `app/store/sqlstore/memory_edge.sql`
- `docs/architecture/memory-knowledge-relationship.md`

## 9. 参考资料

- Hermes Agent `MemoryProvider` 抽象: https://github.com/NousResearch/hermes-agent/blob/main/agent/memory_provider.py
- Hermes Agent memory provider loader: https://github.com/NousResearch/hermes-agent/blob/main/plugins/memory/__init__.py
- Hermes Memory Provider 插件文档: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/developer-guide/memory-provider-plugin.md
- Hermes Memory Providers 用户文档: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/memory-providers.md
- Hermes 贡献说明中关于外部 memory provider 的说明: https://github.com/NousResearch/hermes-agent/blob/main/CONTRIBUTING.md
