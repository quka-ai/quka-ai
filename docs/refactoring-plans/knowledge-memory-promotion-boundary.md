# Knowledge 与 Memory 按需提升边界调整计划

**计划ID**: knowledge-memory-promotion-boundary  
**日期**: 2026-05-25  
**状态**: 已实施  
**优先级**: 高  
**作者**: Codex  

## 0. 2026-09-07 Skill 契约补充

本次补充聚焦 `.agents/skills/openclaw-quka-memory-adapter` 的文档契约，不涉及业务代码修改。

调整目标：

- 将 OpenClaw 适配层的旧假设从“每条 knowledge 都有 matching memory”修正为“knowledge 可按需提升或投影为 memory”。
- 明确 `Knowledge` 是用户拥有的 canonical content，`Memory` 是 agent runtime context。
- 将 `knowledge_id` 在 memory API 返回中的语义限定为 backing/provenance metadata，而不是 memory identity。
- 明确 OpenClaw 适配器必须以 `memory_id` 作为 `recall/get/pin` 等 memory workflow 的稳定身份。
- 补充 promotion 与 projection 规则，避免 agent 通过 memory endpoint 隐式污染用户知识库。

## 1. 背景

原设计中，普通 `knowledge` 创建后会默认注册一条 `semantic/user` memory。这个做法可以让 agent 无感使用用户知识，但会把用户资料库、网页、文档、笔记全部混入 agent runtime 记忆，导致 `recall / hydrate / pin / reflect` 的语义边界变模糊。

本次调整将 `memory` 明确定义为 agent runtime 层面的按需投影：`knowledge` 可以被提升为 `memory`，但不是所有 `knowledge` 都天然拥有 `memory` 身份。

## 2. 改造目标

- 普通用户创建的 `knowledge` 默认只进入知识库、chunk、vector 和 RAG 检索。
- `memory` 仍然复用 `knowledge` 作为正文背板。
- 只有用户、agent 或治理流程显式需要时，才创建 `quka_memory`。
- 保留 `/memory/remember` 传入已有 `knowledge_id` 的提升能力。
- 保留 memory API 不传 `knowledge_id` 时创建隐藏 `__memory__` backing knowledge 的能力。

## 3. 实施方案

1. 调整架构文档，明确：
   - `Memory` 必须依附 `Knowledge`
   - `Knowledge` 不必然拥有 `Memory`
   - 普通知识默认不进入 memory runtime
2. 调整 OpenClaw 适配计划，取消“默认登记所有 knowledge”的表述，改为“按需提升 + 惰性激活”。
3. 调整 `KnowledgeLogic.InsertContent*` 默认行为：
   - `InsertContentAsync`
   - `InsertContentAsyncWithSource`
   - `InsertContent`
4. 保留 `InsertContentAsyncWithSourceWithoutMemory` 作为 memory backing knowledge 创建路径。
5. 保留 `MemoryLogic.RegisterKnowledgeMemory`，作为后续显式提升、兼容或内部治理的能力。

## 4. 关键考虑点

- 删除普通 knowledge 时仍应清理可能指向它的 memory，避免悬挂引用。
- agent 若要长期记住某条用户知识，应通过 `/memory/remember` 携带 `knowledge_id` 完成提升。
- 普通知识仍可通过现有 knowledge/RAG 检索能力被搜索，不依赖 memory catalog。
- UI 后续可以在 knowledge detail 中展示“是否已被 agent 记住”，并提供显式提升入口。

## 5. 相关文件

- `app/logic/v1/knowledge.go`
- `app/logic/v1/memory.go`
- `docs/architecture/memory-knowledge-relationship.md`
- `docs/refactoring-plans/openclaw-memory-minimal-adaptation.md`
- `docs/refactoring-plans/openclaw-memory-architecture.md`
