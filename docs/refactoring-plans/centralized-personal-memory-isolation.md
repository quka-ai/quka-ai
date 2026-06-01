# 中心化个人记忆隔离与空间共享记忆迭代计划

**计划ID**: centralized-personal-memory-isolation  
**日期**: 2026-05-23  
**状态**: 实施中  
**优先级**: 高  
**作者**: Codex  

## 1. 背景

QukaAI 的产品目标是为用户提供生活记忆存储与 RAG 能力，并通过 space 让用户在需要时共享记忆。与 OpenClaw 的本地单用户运行环境不同，QukaAI 是中心化服务，需要同时满足：

- 每个用户拥有独立的个人记忆画像
- 同一用户可在不同 space 中拥有不同上下文
- space 可以承载团队或多人共享记忆
- 共享记忆不能污染个人私有记忆
- 不同用户、不同运行上下文之间不能串用 pin / hydrate / recall 状态

当前代码已经引入 `knowledge + memory` 双层结构：

- `quka_knowledge` 继续作为正文真相源
- `quka_memory` 作为 agent memory runtime 语义层
- `quka_memory_edge` 表达证据、派生、冲突等关系
- `quka_memory_binding` 表达 runtime context 的 pin / working set
- API 已具备 `remember / recall / hydrate / reflect / pin / update / delete`

因此当前不是缺少存储层，而是需要把“中心化多用户隔离 + 空间共享”的边界正式建模并落实到查询、向量召回、绑定和 agent 注入链路。

## 2. 当前具备的能力

### 2.1 已有 memory runtime 雏形

已有核心入口：

- `POST /api/v1/:spaceid/memory/remember`
- `POST /api/v1/:spaceid/memory/recall`
- `POST /api/v1/:spaceid/memory/get`
- `POST /api/v1/:spaceid/memory/hydrate`
- `POST /api/v1/:spaceid/memory/reflect`
- `POST /api/v1/:spaceid/memory/pin`
- `POST /api/v1/:spaceid/memory/update`
- `POST /api/v1/:spaceid/memory/delete`

已有 runtime context 类型：

- `chat_session`
- `agent_run`
- `task`
- `workspace`

已有 memory 类型：

- `core`
- `semantic`
- `episodic`
- `working`

### 2.2 个人隔离已经有第一层基础

`quka_memory`、`quka_knowledge`、`quka_vectors` 都已经有 `user_id` 字段。

当前 `MemoryLogic.Recall` 使用当前登录用户构造可访问记忆：

- `space_id = '-' + scope = user + user_id = 当前用户`
- `space_id = 当前 space + scope = user + user_id = 当前用户`

这说明系统已经具备“同一中心服务内按用户取个人记忆”的基础。

### 2.3 Agent 已能使用 memory 工具

AutoAssistant 已经接入：

- turn 开始前通过 `InjectMemoryContext` 注入相关 memory
- ReAct agent 工具中加入 `SearchUserMemories`
- ReAct agent 工具中加入 `RememberUserMemory`
- ReAct agent 工具中加入 `ForgetUserMemory`

这已经接近 OpenClaw 的 `memory_search / memory_flush / memory_get` 使用方式。

## 3. 当前主要缺口

### 3.1 空间共享记忆被显式关闭

`memory_user_layers.sql` 中当前策略是：

- `user_global`: `space_id = '-'`, `scope = 'user'`, `user_id = owner`
- `user_space`: `space_id = current_space`, `scope = 'user'`, `user_id = owner`
- `scope = 'space'` 的记录会被改成 `scope = 'user'`

这意味着当前系统更偏向“用户在 space 内的个人记忆”，还不具备完整的“space 共享记忆”。

### 3.2 `scope=space` 记忆无法稳定进入 recall

`GetMemoryOptions.Apply` 的可访问条件只包含：

- 当前用户的全局 user memory
- 当前用户在当前 space 的 user memory

即使上层传入 `Scopes: [user, space]`，查询条件也会先被个人可访问条件限制住，导致真正的 `scope=space` 共享记忆不会被召回。

### 3.3 向量召回仍按当前 user_id 过滤

`recallByVector` 在 `knowledgeIDs` 已经来自 memory 可访问集合后，仍然额外按当前 `user_id` 查询 `quka_vectors`。

如果未来允许用户 A 写入 `scope=space` 的共享记忆，用户 B 即使有 space 权限，也无法通过向量召回命中该共享记忆，因为 vector 的 `user_id` 是作者用户。

### 3.4 runtime binding 缺少用户隔离字段

`quka_memory_binding` 当前只有：

- `space_id`
- `memory_id`
- `context_type`
- `context_id`
- `binding_type`
- `pinned_by`

在中心化服务中，外部 agent 的 `agent_run/task/workspace` id 不一定全局唯一。如果两个用户在同一 space 中传入相同 `context_id`，hydrate 可能读取到对方的 working set id。

这对 OpenClaw 本地运行不是问题，但对 QukaAI 中心服务是必须补上的边界。

### 3.5 写入层缺少明确的 layer 规则

当前 `RememberMemory` API 暴露 `scope`，agent tool 暴露 `layer=user_global/user_space`，但还没有一个统一的中心化 layer 语义：

- `user_global`: 个人跨空间记忆
- `user_space`: 个人在某个 space 内的私有记忆
- `space_shared`: 当前 space 内所有有权限成员可使用的共享记忆

没有明确 layer 会导致后续产品和 agent 难以判断“这条记忆是只属于我，还是属于这个空间”。

## 4. 目标模型

建议把中心化 QukaAI memory 拆成三层：

| Layer | DB 表达 | 可见范围 | 典型用途 |
| --- | --- | --- | --- |
| `user_global` | `space_id='-'`, `scope='user'`, `user_id=<owner>` | 仅 owner，跨 space 生效 | 用户偏好、身份信息、长期生活习惯 |
| `user_space` | `space_id=<space>`, `scope='user'`, `user_id=<owner>` | 仅 owner，在该 space 生效 | 某个项目/家庭/场景下的个人上下文 |
| `space_shared` | `space_id=<space>`, `scope='space'` | 有该 space view 权限的成员 | 团队共识、共享资料、多人协作记忆 |

这样既保留“空间共享”，也能保证“人与人之间的个人记忆不相互干扰”。

## 5. 第一批迭代建议

### 5.1 恢复并严格定义 `space_shared`

新增 access option：

- `IncludeSpaceSharedMemory`

召回时可访问范围改为：

- 当前用户的 `user_global`
- 当前用户的 `user_space`
- 当前 space 的 `space_shared`

更新/删除规则：

- `user_global/user_space` 只能 owner 修改
- `space_shared` 需要当前用户具备 space edit 权限

### 5.2 为 binding 增加用户边界

给 `quka_memory_binding` 增加 `user_id` 字段。

建议规则：

- `chat_session` binding 必须绑定 session owner
- `agent_run/task/workspace` 默认绑定当前用户
- 后续如需共享 workspace working set，再显式增加 `binding_scope=space`

第一版先让所有 binding 默认 user-scoped，避免中心化服务串上下文。

### 5.3 调整 memory 向量召回

`recallByVector` 已经拿到了预过滤的 `memory -> knowledge_id` 集合，因此向量查询可以优先依赖：

- `space_id in ['-', current_space]`
- `knowledge_id in accessible_knowledge_ids`

不应再用当前用户 `user_id` 排除 `space_shared` 的作者向量。

### 5.4 统一 API 与 agent tool 的 layer 语义

建议在 handler 层新增可选字段：

- `layer`: `user_global | user_space | space_shared`

兼容现有 `scope` 字段，但内部优先把 `layer` 解析为明确的 `space_id + scope`。

agent 工具新增 `space_shared` 选项，但系统提示词必须约束：

- 默认写入 `user_space`
- 只有用户明确要求共享给 space，或内容明显是多人共同约定，才写入 `space_shared`
- 个人偏好、生活习惯、身份信息不得写入 `space_shared`

### 5.5 增加隔离测试

至少补充以下测试：

- 用户 A 的 `user_global` 不会被用户 B recall
- 同一 space 中用户 A 的 `user_space` 不会被用户 B recall
- 同一 space 中 `space_shared` 能被用户 A/B recall
- `agent_run` 相同 `context_id` 下，用户 A/B 的 pin/hydrate 不串
- `scope=space` 的作者向量能被其他 space 成员召回

## 6. 实施步骤

### 阶段一：安全边界修正

1. 扩展 `GetMemoryOptions`，加入 `IncludeSpaceSharedMemory`
2. 更新 `GetMemoryOptions.Apply` 的可访问条件
3. 修正 `MemoryLogic.Recall/findAccessibleMemory` 的访问模型
4. 调整 `recallByVector`，移除对当前 user_id 的过度过滤
5. 补充 store 层或 logic 层单元测试

### 阶段二：binding 用户隔离

1. 给 `MemoryBinding` 类型增加 `UserID`
2. 给 `quka_memory_binding` 和迁移增加 `user_id`
3. `Pin/Hydrate/Reflect/deleteMemoryReferences` 按当前用户过滤 binding
4. 补充相同 `context_id` 不串用户的测试

### 阶段三：产品语义收敛

1. 为 API 增加 `layer`
2. 为 agent memory tool 增加 `space_shared`
3. 更新 memory system prompt
4. 明确前端 UI 中个人记忆与空间共享记忆的展示区分

## 7. 第一批代码切片

### 7.1 Memory access predicate

目标文件：

- `pkg/types/memory.go`
- `app/logic/v1/memory.go`

拟调整内容：

1. 在 `GetMemoryOptions` 中增加：
   - `IncludeSpaceSharedMemory bool`
2. `GetMemoryOptions.Apply` 中当 `AccessibleUserID` 存在时，访问条件应为：
   - `space_id = '-' AND scope = 'user' AND user_id = AccessibleUserID`
   - `space_id = AccessibleSpaceID AND scope = 'user' AND user_id = AccessibleUserID`
   - `space_id = AccessibleSpaceID AND scope = 'space'`
3. `MemoryLogic.Recall` 默认开启：
   - `IncludeGlobalUserMemory`
   - `IncludeSpaceUserMemory`
   - `IncludeSpaceSharedMemory`
4. `MemoryLogic.findAccessibleMemory` 也使用同一访问模型，避免 update/delete 绕过共享边界。

验收点：

- 同一 space 中，用户 B 不能 recall 用户 A 的 `scope=user` memory
- 同一 space 中，用户 B 可以 recall `scope=space` memory

### 7.2 Vector recall access alignment

目标文件：

- `app/logic/v1/memory.go`

拟调整内容：

1. `recallByVector` 保留 `KnowledgeIDs` 白名单
2. `recallByVector` 不再强制传当前用户 `UserID`
3. 向量查询仍限制在 `accessibleMemorySpaceIDs(spaceID)` 内

原因：

- memory 列表已经完成访问预过滤
- 再按当前用户过滤 vector 会排除其他成员写入的 `space_shared` memory

验收点：

- 用户 A 创建 `scope=space` memory 后，用户 B 可以通过 query 向量命中该 memory
- 用户 B 仍不能命中用户 A 的 `scope=user` memory，因为对应 knowledge_id 不在预过滤白名单中

### 7.3 Binding user boundary

目标文件：

- `pkg/types/memory.go`
- `app/store/sqlstore/memory_binding.go`
- `app/store/sqlstore/memory_binding.sql`
- `app/store/sqlstore/migrations/*`
- `app/logic/v1/memory.go`

拟调整内容：

1. `MemoryBinding` 增加 `UserID string`
2. `GetMemoryBindingOptions` 增加 `UserID string`
3. `quka_memory_binding` 增加 `user_id VARCHAR(32) NOT NULL DEFAULT ''`
4. `Pin` 创建 binding 时写入当前用户
5. `Hydrate` 查询 binding 时按当前用户过滤
6. `deleteMemoryReferences` 删除 binding 时按 memory 所属空间和当前访问规则执行，避免误删其他用户同名 context 的 binding

验收点：

- 用户 A 和用户 B 在同一 space 中使用相同 `agent_run.id` 时，hydrate 的 working memories 不互相出现
- 删除用户 A 的 memory 不会误删用户 B 的同名 context binding

### 7.4 Layer write semantics

目标文件：

- `cmd/service/handler/memory.go`
- `app/logic/v1/memory.go`
- `pkg/ai/agents/memory/function.go`
- `app/logic/v1/memory_prompt.go`

拟调整内容：

1. API request 增加可选 `layer`
2. 内部新增解析函数：
   - `user_global` -> `space_id='-'`, `scope='user'`
   - `user_space` -> `space_id=current_space`, `scope='user'`
   - `space_shared` -> `space_id=current_space`, `scope='space'`
3. agent tool 允许 `space_shared`，但默认仍为 `user_space`
4. prompt 增加约束：个人偏好、生活习惯、身份信息不得写入 `space_shared`

验收点：

- 不传 `layer` 的现有 API 兼容旧行为
- agent 只有在明确共享语义时才写入 `space_shared`

## 8. 验证记录

### 2026-05-23 第一批边界修正

已完成：

- `GetMemoryOptions` 增加 `IncludeSpaceSharedMemory`
- recall / update / delete / pin 使用统一可访问 memory 模型
- `recallByVector` 改为依赖预过滤后的 `knowledge_ids`，不再用当前 `user_id` 排除共享记忆作者向量
- `quka_memory_binding` 增加 `user_id`，hydrate / pin / reflect 按当前用户过滤 runtime binding
- API 和 agent tool 增加 `user_global / user_space / space_shared` layer 语义
- 自动 reflect 默认写入 `user_space`，避免私有会话总结进入共享空间
- 增加无需数据库的 predicate 单元测试

已运行：

```bash
go test ./pkg/types ./pkg/ai/agents/memory/...
go test ./app/store/sqlstore -run '^$'
go test ./app/logic/v1 -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./pkg/ai/agents/... -run '^$'
```

结果：

- 以上验证均通过
- 其中 `-run '^$'` 用于编译验证，避开当前仓库依赖本地数据库或 fixture 的旧测试

### 2026-05-25 Knowledge 不再默认进入 Memory Runtime

已完成：

- 调整目标模型：`memory` 是 agent runtime 层面的按需投影，不是普通 `knowledge` 的默认副产物
- 普通 `KnowledgeLogic.InsertContent*` 创建 knowledge 后不再默认注册 `quka_memory`
- 保留 `/memory/remember` 传入 `knowledge_id` 时把已有 knowledge 提升为 memory 的能力
- 保留 memory API 创建 `__memory__` hidden backing knowledge 的能力
- 更新架构与 OpenClaw 适配文档，统一为“按需提升 + 惰性激活 + 渐进治理”

影响：

- 旧的“Knowledge 默认进入 Memory Runtime”结论已被本节取代
- 普通知识仍可通过 knowledge/RAG 检索使用
- agent 长期记忆只来自显式 remember、pin、reflect 或后台治理提升

### 2026-05-23 Knowledge 默认进入 Memory Runtime

> 该阶段结论已于 2026-05-25 被“Knowledge 不再默认进入 Memory Runtime”取代，保留本节作为历史记录。

已完成：

- 普通 `KnowledgeLogic.InsertContent*` 创建 knowledge 后默认注册对应 `user_space` memory
- `MemoryLogic.Remember` 创建 backing knowledge 时跳过默认注册，避免先生成默认 semantic/user memory 再覆盖 `core/episodic/layer` 语义
- `RegisterKnowledgeMemory` 增加默认值保护：
  - 空 `author_type` 默认 `human`
  - 空 `source_kind` 默认 `manual`
- 增加 knowledge source 到 memory source / author 的映射：
  - platform -> manual / human
  - chat -> chat / agent
  - mcp -> mcp / agent
  - rss -> rss / system
  - podcast -> manual / system
- 增加 `ResolveMemoryLayer` 和 source mapping 的无数据库测试

已运行：

```bash
go test ./app/logic/v1 -run 'TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1 -run '^$'
go test ./pkg/types ./pkg/ai/agents/memory/...
go test ./cmd/service/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过
- 全量 `go test ./...` 仍需本地 PostgreSQL/pgvector 和现有 sqlstore fixture，暂未作为本阶段完成条件

### 2026-05-23 Memory 写路径权限收紧

已完成：

- `MemoryLogic.Update/Delete/Forget` 在执行前统一检查 mutation 权限
- `scope=user` memory 只能 owner 修改或删除
- `scope=space` memory 需要当前用户在该 space 具备 `edit` 权限
- `space_id='-'` 的 `scope=space` 被视为无效写目标
- 增加无数据库测试覆盖：
  - owner 可以修改个人 memory
  - space editor 不能修改他人的个人 memory
  - space shared memory 需要 edit 权限
  - global shared memory 不能被修改

已运行：

```bash
go test ./app/logic/v1 -run 'TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1 -run '^$'
go test ./pkg/types
go test ./cmd/service/... -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 Hydrate 装配 Working Set

已完成：

- `Hydrate` 不再只返回 pinned / working memory IDs
- `Hydrate` 会按当前用户和当前 space 的访问模型重新加载 runtime binding 中的 memory 正文
- 不可访问、已删除或过期的 binding memory 不会进入 hydration context
- working memory 在 `AssembledContext` 中排在普通 recall 结果之前
- working memory 与 recall 命中相同 memory 时只装配一次
- 增加 `formatMemoryContextPart` 的无数据库测试，锁定 hydrate context 的基本格式与 title fallback

已运行：

```bash
go test ./app/logic/v1 -run 'TestFormatMemoryContextPart|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1 -run '^$'
go test ./pkg/types
go test ./cmd/service/... -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 内置聊天接入 Hydrate

已完成：

- `InjectMemoryContext` 从直接 `Recall` 改为调用 `Hydrate`
- 有 `session_id` 时使用 `runtime_context.type=chat_session`
- 无 `session_id` 时仍可通过 hydrate 的普通 recall 装配 memory context
- 内置聊天现在可以加载当前 chat session 的 pinned / working memories
- memory context block 直接使用 `Hydrate.AssembledContext`，避免绕过 working set
- 增加 `buildHydratedMemoryContextBlock` 的无数据库测试

已运行：

```bash
go test ./app/logic/v1 -run 'TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1 -run '^$'
go test ./pkg/types
go test ./cmd/service/... -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 Recall API 暴露 Memory Layer 元信息

已完成：

- 增加统一 helper `MemoryLayerOf`
- `/memory/recall` 响应 item 增加：
  - `space_id`
  - `layer`
  - `scope`
  - `entity_key`
  - `author_type`
  - `epistemic_status`
  - `source_kind`
  - `source_ref`
- 外部 agent / 前端可以直接区分：
  - `user_global`
  - `user_space`
  - `space_shared`
- 增加 `MemoryLayerOf` 的无数据库测试

已运行：

```bash
go test ./app/logic/v1 -run 'TestMemoryLayerOf|TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./cmd/service/... -run '^$'
go test ./app/logic/v1 -run '^$'
go test ./pkg/types
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 Hydrate 返回结构化 Memory Pack

已完成：

- `HydrateMemoryResult` 增加 `items`
- `items` 按实际装配顺序返回：
  - `role=working`
  - `role=recall`
- 每个 item 返回与 recall 一致的关键元信息：
  - `memory_id`
  - `knowledge_id`
  - `space_id`
  - `layer`
  - `scope`
  - `memory_type`
  - `title`
  - `content`
  - `entity_key`
  - `confidence`
  - `importance`
  - `author_type`
  - `epistemic_status`
  - `source_kind`
  - `source_ref`
- `assembled_context` 保持兼容旧客户端
- 外部 agent 可以直接使用 `items` 自行装配上下文，而不是解析 markdown 文本
- 增加 `buildHydrateMemoryContextItem` 的无数据库测试

已运行：

```bash
go test ./app/logic/v1 -run 'TestBuildHydrateMemoryContextItem|TestMemoryLayerOf|TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1 -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/types
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 Hydrate 支持 Token Budget

已完成：

- `HydrateMemoryArgs.TokenBudget` 开始参与装配流程
- hydrate 先形成有序 `items`，再按预算过滤，最后反推：
  - `working_memories`
  - `core_memories`
  - `semantic_memories`
  - `recent_episodic_memories`
  - `assembled_context`
- working memories 排在 recall memories 前面，因此预算不足时优先保留 pinned working set
- `items` 和 `assembled_context` 使用同一批预算内 memory，避免结构化返回和文本上下文不一致
- 当前采用轻量近似 token 估算：约 4 个 rune 计为 1 token
- 增加无数据库测试覆盖：
  - 预算不足时优先保留 working item
  - hydrate 视图可从最终 items 重新构建

已运行：

```bash
go test ./app/logic/v1 -run 'TestApplyHydrateTokenBudgetPrioritizesWorkingItems|TestHydrateResultRebuildHydrateViews|TestBuildHydrateMemoryContextItem|TestMemoryLayerOf|TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1 -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/types
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 Binding 迁移回填与重复 Pin 防护

已完成：

- `memory_binding_user_isolation.sql` 增加历史数据回填：
  - 通过 `memory_id -> quka_memory.id` 将旧 binding 的 `user_id` 补齐为 memory owner
- 增加迁移内去重：
  - 同一 `space_id/user_id/memory_id/context_type/context_id/binding_type` 仅保留最新一条
- 增加唯一索引：
  - `idx_quka_memory_binding_unique_context_memory`
- `MemoryBindingStore.Create` 增加 `ON CONFLICT (...) DO NOTHING`
  - 并发或重复 pin 同一 memory/context 时不会插入重复 binding，也不会因唯一冲突失败
- `memory_binding.sql` 与 `memory_foundation.sql` 同步唯一索引

已运行：

```bash
go test ./app/store/sqlstore -run '^$'
go test ./app/logic/v1 -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/types
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 Knowledge 删除同步清理 Memory

已完成：

- `KnowledgeLogic.Delete` 在同一事务内清理对应 memory runtime 记录
- 删除顺序覆盖：
  - `quka_knowledge`
  - `quka_knowledge_chunk`
  - `quka_vectors`
  - `quka_memory_binding`
  - `quka_memory_edge`
  - `quka_memory`
- 避免普通 knowledge 默认注册 memory 后，删除 knowledge 留下悬挂 memory 污染 recall / hydrate
- `KnowledgeLogic.Update` 不需要额外同步正文，因为 memory 指向同一条 canonical knowledge

已运行：

```bash
go test ./app/logic/v1 -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/types
go test ./app/store/sqlstore -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 当前工作区验证

已运行：

```bash
go test ./app/logic/v1/...
go test ./pkg/ai/agents/memory/... ./app/store/sqlstore/...
```

结果：

- `pkg/ai/agents/memory` 当前无测试文件，可编译阶段未暴露问题
- `app/logic/v1` 测试失败于本地 PostgreSQL/pgvector 未启动：`dial tcp [::1]:5432: connect: connection refused`
- `app/store/sqlstore` 测试失败于现有 fixture 缺失：`open ./test_vectors: no such file or directory`

说明：

- 本次失败不是 memory 隔离逻辑断言失败
- 后续应补充无需真实数据库的 predicate 单元测试，或引入可控 testcontainer / fixture，避免中心化隔离能力只能依赖人工代码阅读

### 2026-05-23 批量删除与整空间删除的 Memory 生命周期补齐

已完成：

- `MemoryStore` 增加按 `space_id + memory_ids` 批量删除能力
- `MemoryBindingStore` / `MemoryEdgeStore` 增加按 memory id 批量清理引用能力
- `MemoryStore.List`、`MemoryBindingStore.List`、`MemoryEdgeStore.List` 支持 `NO_PAGINATION`
- `KnowledgeLogic.deleteRegisteredMemoriesByKnowledgeIDs` 改为批量清理：
  - 先找出 knowledge 对应的 memory
  - 再删除 binding / edge
  - 最后删除 memory 本体
- `ResourceLogic.Delete` 删除资源下所有 knowledge 时同步清理 memory runtime 数据
- `AIFileDisposeLogic.DeleteTask` 删除任务生成的 knowledge 时同步清理 memory runtime 数据
- `ExpirationCleanupTask.hardDelete` 清理过期 knowledge 时同步清理 memory runtime 数据
- `SpaceLogic.DeleteUserSpace` 与 `AdminUserLogic.deleteSpaceKnowledgeData` 整空间删除时同步删除：
  - `quka_memory_binding`
  - `quka_memory_edge`
  - `quka_memory`

价值：

- 避免 knowledge 已删除但 memory 仍可被 recall / hydrate 命中
- 避免 pin / working set 引用已经不存在的 memory
- 让中心化服务中的个人记忆隔离不被历史孤儿数据破坏

已运行：

```bash
go test ./app/logic/v1 -run 'TestApplyHydrateTokenBudgetPrioritizesWorkingItems|TestHydrateResultRebuildHydrateViews|TestBuildHydrateMemoryContextItem|TestMemoryLayerOf|TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1 -run '^$'
go test ./app/logic/v1/process -run '^$'
go test ./app/logic/v1/... -run '^$'
go test ./app/store/sqlstore -run '^$'
go test ./pkg/types
go test ./cmd/service/... -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 原生 `/memory/get` 读取入口

已完成：

- 新增 `POST /api/v1/:spaceid/memory/get`
- 请求支持：
  - `memory_id`
  - 兼容字段 `id`
- 返回结构复用 `/memory/recall` 的 item 结构，包含：
  - `memory_id`
  - `knowledge_id`
  - `space_id`
  - `layer`
  - `scope`
  - `memory_type`
  - canonical `content`
  - confidence / importance / source 等元信息
- 读取前统一经过 `MemoryLogic.findAccessibleMemory`
  - 当前用户可读自己的 `user_global`
  - 当前用户可读自己的 `user_space`
  - space 成员可读当前 space 的 `space_shared`
  - 不允许通过 `knowledge_id` 直接绕过 memory 访问边界
- 对 OpenClaw adapter skill 而言，`memory_get` 不再需要先猜 backing knowledge 所在 `space_id`
  - 尤其解决 `user_global` 的 backing knowledge 存在于 `space_id='-'` 时，从普通 space 接入不自然的问题

同时补充：

- `GetMemoryOptions.Apply` 的 layer flag 单元测试：
  - 只开 `IncludeSpaceSharedMemory` 时只包含 `space_shared`
  - 只开 private user layers 时不包含 `space_shared`
  - 只开 global user 时不包含当前 space 记忆
- `docs/refactoring-plans/openclaw-memory-api-adapter.md` 更新 `memory_get` contract

已运行：

```bash
go test ./pkg/types
go test ./app/logic/v1 -run 'TestApplyHydrateTokenBudgetPrioritizesWorkingItems|TestHydrateResultRebuildHydrateViews|TestBuildHydrateMemoryContextItem|TestMemoryLayerOf|TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1/... -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
go test ./pkg/types ./app/store/sqlstore -run '^$'
```

结果：

- 以上验证均通过

### 2026-05-23 Remember 写入与更新权限边界收紧

已完成：

- `MemoryLogic.Remember` 新增创建权限检查：
  - `scope=user` / 默认 scope 可创建个人记忆
  - `scope=space` 只能在非全局 space 中由具备 edit 权限的用户创建
  - `scope=agent/session` 等非当前产品化层级不允许通过 remember 创建
- `Remember` 命中已有 memory 时，改为使用真实 memory 记录执行 mutation 权限判断：
  - 另一个用户不能通过已知 `knowledge_id` 更新对方的 `user_space` / `user_global` 私有 memory
  - `space_shared` 仍需 space edit 权限才能更新
- 增加 `canCreateMemory` 的无数据库测试，锁定上述边界

价值：

- 避免中心化服务中“知道 id 即可改 metadata”的越权路径
- 让 agent tool、HTTP API、内部 reflect 统一走同一套写入边界
- 对 OpenClaw-like 外部客户端更安全：客户端可以传 `memory_id/knowledge_id`，但不能越过服务端个人隔离规则

已运行：

```bash
go test ./app/logic/v1 -run 'TestCanCreateMemory|TestCanMutateMemory|TestResolveMemoryLayer|TestMemoryLayerOf|TestKnowledgeSourceMemoryDefaults'
```

结果：

- 验证通过

### 2026-05-23 Runtime Context 校验收紧

已完成：

- 增加统一 `validateRuntimeContext`
- `Hydrate`：
  - 允许 `runtime_context=nil`，表示只做普通 recall 装配，不读取 pinned / working memories
  - 如果传入 `runtime_context`，必须包含合法 `type` 和非空 `id`
- `Pin` / `Reflect`：
  - 强制要求合法 `runtime_context.type`
  - 强制要求非空 `runtime_context.id`
- 服务端会 trim `runtime_context.id`，避免首尾空白造成同一 run/task/workspace 被拆成两个 binding namespace
- 合法类型限制为：
  - `chat_session`
  - `agent_run`
  - `task`
  - `workspace`
- 增加 `TestValidateRuntimeContext` 无数据库测试

价值：

- 防止外部 OpenClaw-like client 传空 context id，导致同一用户自己的多个 run/task/workspace 混用 working set
- 让 center-hosted runtime context 具备比本地单用户 OpenClaw 更明确的服务端边界
- 与 `quka_memory_binding.user_id` 共同形成 `user_id + context_type + context_id` 的隔离坐标

已运行：

```bash
go test ./app/logic/v1 -run 'TestValidateRuntimeContext|TestCanCreateMemory|TestCanMutateMemory'
go test ./app/logic/v1 -run 'TestApplyHydrateTokenBudgetPrioritizesWorkingItems|TestHydrateResultRebuildHydrateViews|TestBuildHydrateMemoryContextItem|TestMemoryLayerOf|TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestValidateRuntimeContext|TestCanCreateMemory|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1/... -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/types ./app/store/sqlstore -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
```

结果：

- 验证通过

### 2026-05-23 全局个人记忆删除的跨 Space 引用清理

已完成：

- 修正 `MemoryBindingStore.DeleteByMemoryIDs`
  - 普通 space memory：继续按 `space_id + memory_id` 清理
  - `user_global` memory：按 `memory_id` 跨 space 清理 binding
- 修正 `MemoryEdgeStore.DeleteByMemoryIDs`
  - 普通 space memory：继续按 `space_id + from/to_memory_id` 清理
  - `user_global` memory：按 `from/to_memory_id` 跨 space 清理 edge
- `MemoryLogic.deleteMemoryReferences` 改为复用上述批量清理能力
  - hard delete / forget 单条 `user_global` memory 时，同样跨 space 清理 runtime 引用
  - 与 knowledge/resource/task/expiration cleanup 的批量删除路径保持一致
- 增加 SQL predicate 单元测试，锁定：
  - 全局 memory 清理不能带 `space_id` 条件
  - 普通 space memory 清理必须带 `space_id` 条件

价值：

- 用户全局记忆可以被 pin 到不同 space 的 runtime context
- 删除全局个人记忆时，不会在某个 space 的 working set / edge graph 中留下悬挂引用
- 普通 space 记忆仍保持 space-scoped 删除，避免误删其他 space 的 runtime 引用

已运行：

```bash
go test ./app/store/sqlstore -run 'TestDeleteMemory.*ByMemoryIDsQuery'
go test ./app/logic/v1 -run 'TestCanMutateMemory|TestCanCreateMemory|TestValidateRuntimeContext'
go test ./app/logic/v1 -run 'TestApplyHydrateTokenBudgetPrioritizesWorkingItems|TestHydrateResultRebuildHydrateViews|TestBuildHydrateMemoryContextItem|TestMemoryLayerOf|TestBuildHydratedMemoryContextBlock|TestFormatMemoryContextPart|TestValidateRuntimeContext|TestCanCreateMemory|TestCanMutateMemory|TestResolveMemoryLayer|TestKnowledgeSourceMemoryDefaults'
go test ./app/logic/v1/... -run '^$'
go test ./cmd/service/... -run '^$'
go test ./pkg/types ./app/store/sqlstore -run '^$'
go test ./pkg/ai/agents/... -run '^$'
go test ./pkg/mcp/... -run '^$'
```

结果：

- 验证通过

### 2026-05-23 OpenClaw Skill-first 接入决策

已确认：

- 不新增 OpenClaw 专用兼容 HTTP 入口
- 不保留未被服务端使用的 OpenClaw 专用逻辑 adapter
- OpenClaw-like agent 通过 skills 能力完成语义映射
- QukaAI server 继续只维护原生 memory API：
  - `/memory/recall`
  - `/memory/get`
  - `/memory/remember`
  - `/memory/hydrate`
  - `/memory/pin`
  - `/memory/reflect`
  - `/memory/update`
  - `/memory/delete`

价值：

- 避免 OpenClaw-specific route / handler 与原生 memory API 长期并存
- 让 agent 侧 skill 承担 `memory_search / memory_get / memory_flush` 的语义翻译
- 服务端专注中心化隔离、生命周期、权限和 RAG 能力
- 不新增第二套 source of truth，也不新增第二套 API surface

文档同步：

- `docs/refactoring-plans/openclaw-memory-api-adapter.md` 已改为 skill-first 接入边界
- `.agents/skills/openclaw-quka-memory-adapter` 已补充 center-hosted 写入规则：
  - agent 应显式选择 `user_global / user_space / space_shared`
  - `user_space` 是默认安全选择
  - `space_shared` 仅用于明确共享或团队/项目共同约定
  - 个人偏好、生活细节、身份事实、私密摘要、secret / credential、敏感观察不得写入 `space_shared`
- OpenClaw 语义映射继续留在 skill / 文档层，服务端不新增 OpenClaw 专用 route / handler / adapter

### 2026-05-23 向量召回隔离防回归测试

已完成：

- 将 memory vector recall 的查询参数组装收敛为 `buildMemoryRecallVectorOptions`
- 增加 `TestBuildMemoryRecallVectorOptionsUsesAccessibleKnowledgeOnly`
  - 输入包含当前用户私有 `user_space`
  - 输入包含其他用户写入的 `space_shared`
  - 输入包含当前用户 `user_global`
  - 断言 vector query 不再携带 `UserID`
  - 断言 vector query 只使用 `accessibleMemorySpaceIDs(spaceID)` 和已经过 memory access prefilter 的 `knowledge_ids`
- 增加 `TestBuildMemoryRecallVectorOptionsRejectsEmptyKnowledgeSet`

价值：

- 锁定“memory access predicate 先决定可访问集合，vector store 只负责在这个集合内排序/命中”的边界
- 防止未来重新按当前 `user_id` 查询 vector，导致其他成员写入的 `space_shared` memory 无法被同 space 成员向量召回
- 与中心化服务的隔离目标一致：私有 memory 先被 access predicate 排除，共享 memory 不应再被作者 `user_id` 错误排除

已运行：

```bash
go test ./app/logic/v1 -run 'TestBuildMemoryRecallVectorOptions|TestResolveMemoryLayer|TestMemoryLayerOf'
go test ./app/logic/v1/... -run '^$'
go test ./pkg/types ./app/store/sqlstore -run '^$'
```

结果：

- 验证通过

### 2026-05-23 Memory API 入口权限与 hydrate 语义对齐

已完成：

- `HydrateMemoryRequest.runtime_context` 去掉 `binding:"required"`
  - 与 `MemoryLogic.Hydrate` 的语义保持一致：不传 runtime context 时，只做普通 recall 装配，不读取 pinned / working set
  - 增加 `TestHydrateMemoryRequestRuntimeContextIsOptional` 防止入口层再次把 optional context 收紧为 required
- `/memory/pin` 从 edit scope 移到 view scope
  - pin 写入的是当前用户自己的 `quka_memory_binding.user_id + runtime_context` working set
  - pin 前仍通过 `findAccessibleMemory` 校验 memory 可访问边界
  - `Recall` 在 view scope 下同样会更新访问统计，因此 pin 的访问统计更新不应要求 space edit
- `/memory/remember / reflect / update / delete` 仍保留在 edit scope
  - 其中 `space_shared` 创建和修改继续由 logic 层二次检查 space edit 权限
  - 私有 memory 的 owner-only mutation 仍由 `ensureCanMutateMemory` 兜底

价值：

- view 成员可以维护自己的 runtime working set，不需要拥有 space 内容编辑权
- 外部 agent 可以按 `hydrate -> recall -> pin` 建立当前用户自己的工作记忆，不会和其他用户共享 binding
- hydrate 的 HTTP contract 与 skill / 文档 / logic 保持一致，避免 nil runtime context 被入口层提前拒绝

已运行：

```bash
go test ./cmd/service/handler -run 'TestHydrateMemoryRequestRuntimeContextIsOptional|TestKnowledgeToKnowledgeResponseLite'
go test ./cmd/service/... -run '^$'
```

结果：

- 验证通过

### 2026-05-23 DB 层中心化隔离约束

已完成：

- 在 `quka_memory` schema 中增加 `chk_quka_memory_layer_scope`
  - `scope='user'` 必须携带非空 `user_id`
  - `scope='space'` 必须落在非空且非 `'-'` 的真实 space
  - 从 DB 层阻止 `space_id='-' + scope='space'` 这类无效共享层写入
- 在 `quka_memory_binding` schema 中增加 `chk_quka_memory_binding_user_id`
  - 新写入 binding 必须携带非空 `user_id`
  - 与中心化 runtime context 隔离模型一致：`space_id + user_id + context_type + context_id`
- 新增 `memory_layer_constraints.sql`
  - 使用 `NOT VALID` 添加约束，避免部署时被历史数据阻塞
  - PostgreSQL 会对约束添加后的新写入和更新继续执行检查
- 增加 `memory_schema_test.go`
  - 锁定 schema 和 migration 中必须存在上述约束

价值：

- memory layer 的合法形态不再只依赖应用层 helper
- 即使未来新增写入口，也更难写入全局共享 memory 或无用户归属 binding
- 对中心化服务尤其重要：避免异常数据绕过 `user_global / user_space / space_shared` 三层模型

已运行：

```bash
go test ./app/store/sqlstore -run 'TestMemorySchemaEnforcesCenterHostedLayerConstraints|TestMemoryLayerConstraintMigrationUsesNotValid|TestDeleteMemory.*ByMemoryIDsQuery'
```

结果：

- 验证通过

### 2026-05-23 Memory Backing Knowledge 普通知识入口隔离

已完成：

- 新增内部保留 resource：`types.MEMORY_BACKING_RESOURCE = "__memory__"`
- `MemoryLogic.Remember` 新建 backing knowledge 时统一使用 `MEMORY_BACKING_RESOURCE`
  - 不再因为 agent tool / reflect 传入默认 resource 而落到普通 `knowledge` 列表
  - 如果调用方传入已有 `knowledge_id`，仍按已有 knowledge 建立 memory，不改变原 knowledge resource
- 普通 `KnowledgeLogic.ListUserKnowledges` 默认排除 `MEMORY_BACKING_RESOURCE`
  - 即使请求显式包含 `__memory__`，也会从 include 过滤掉
  - 只请求 `__memory__` 时返回空列表
- 普通 `GetKnowledge / Update / Delete` 对 `MEMORY_BACKING_RESOURCE` 返回 not found
  - memory 正文读取走 `/memory/get`
  - memory 修改和删除走 `/memory/update` / `/memory/delete`

价值：

- 解决共享 space 中私有 `user_space` memory 的 backing knowledge 被普通 knowledge list 暴露给其他成员的风险
- 保持人类知识库列表与 agent memory runtime 的访问边界分离
- 对 `space_shared` memory 也保持统一：共享记忆通过 memory API 暴露，而不是绕过 memory access predicate 直接读 backing knowledge

已运行：

```bash
go test ./app/logic/v1 -run 'TestFilterMemoryBackingResource|TestIsMemoryBackingKnowledge|TestBuildMemoryRecallVectorOptions|TestValidateRuntimeContext|TestCanCreateMemory|TestCanMutateMemory'
go test ./pkg/types -run '^$'
```

结果：

- 验证通过

## 9. 需要确认的问题

1. 普通 space editor 是否可以创建 `space_shared` memory，还是必须 maintainer/admin 才能创建？
2. `space_shared` memory 是否允许所有 view 成员 recall，还是需要额外开关？
3. 用户手工创建普通 knowledge 时，默认注册为 `user_space`，还是由 UI 选择是否共享为 `space_shared`？
4. agent 自动 reflect 的结果默认应为 `user_space`，还是保留当前的 `scope=space`？
5. 删除 `space_shared` memory 时，是否需要记录审计日志或软删除优先？

## 10. 当前结论

当前 QukaAI 已经具备 OpenClaw 风格 memory runtime 的基础骨架，并且第一批中心化隔离边界已经落地，尤其是：

- memory API 已成形
- agent tool 已接入
- memory 与 knowledge 已分层
- 个人 `user_global/user_space` 隔离已有基础并已接入 recall / hydrate / mutate
- `space_shared` 已具备受控召回与编辑权限边界
- binding 已加入 `user_id` 隔离，降低中心化 runtime context 串用风险
- knowledge / resource / task / 过期清理 / 整空间删除已同步清理 memory 生命周期

但还不能说已经完整具备“中心化千人千面 memory 体系”，原因是仍缺少：

- 基于真实 PostgreSQL/pgvector 的隔离集成测试
- `space_shared` 的产品级 UI / 审计 / 可见性确认
- 大规模 memory lifecycle 的迁移演练与历史数据校验
- 通过 skill 调用原生 `/memory/*` API 的 OpenClaw-like 端到端验证

下一步应优先补齐集成验证与 skill 调用链验证，然后再推进前端产品化展示与运营审计能力。
