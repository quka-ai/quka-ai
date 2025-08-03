# AutoAssistant 基于 Eino 框架的改造计划

## 📋 项目背景

### 现状分析
- **现有实现**: `NormalAssistant` 使用自定义的 AI 工作流和工具调用机制
- **改造目标**: 创建 `AutoAssistant`，集成 eino 框架，保持接口兼容性
- **核心要求**: 不修改现有代码，新建结构体实现相同接口

### NormalAssistant 核心方法分析
```go
type NormalAssistant struct {
    core      *core.Core
    agentType string
}

// 核心接口方法
func (s *NormalAssistant) InitAssistantMessage(ctx context.Context, msgID string, seqID int64, userReqMessage *types.ChatMessage, ext types.ChatMessageExt) (*types.ChatMessage, error)
func (s *NormalAssistant) GenSessionContext(ctx context.Context, prompt string, reqMsgWithDocs *types.ChatMessage) (*SessionContext, error) 
func (s *NormalAssistant) RequestAssistant(ctx context.Context, reqMsg *types.ChatMessage, receiver types.Receiver, aiCallOptions *types.AICallOptions) error
```

### 现有工作流程
```
用户请求 → 生成上下文 → requestAIWithTools → 工具调用循环 → 流式响应 → 完成
```

## 🎯 改造设计方案

### 核心新增功能：工具调用持久化

**问题**: 现有 NormalAssistant 中工具调用只通过 ToolTips 实时推送，不保存到数据库，前端刷新后工具调用记录丢失

**解决方案**: 在 AutoAssistant 中将工具调用过程作为 `role=tool` 的聊天记录保存到数据库

**设计要点**:
- 工具调用开始时保存一条 `role=tool` 记录，状态为 `running`
- 工具调用完成时更新记录，包含完整的参数和结果
- 保持 ToolTips 实时推送的同时，增加数据库持久化
- 前端可以通过聊天历史展示完整的工具调用过程

### 1. AutoAssistant 结构体设计

**目标**: 完全兼容 NormalAssistant 接口，内部使用 eino 框架，增加工具调用持久化

```go
type AutoAssistant struct {
    core      *core.Core
    agentType string
    // 新增 eino 相关配置
}

// 保持接口兼容性
func (a *AutoAssistant) InitAssistantMessage(...) (*types.ChatMessage, error)
func (a *AutoAssistant) GenSessionContext(...) (*SessionContext, error) 
func (a *AutoAssistant) RequestAssistant(...) error
```

### 2. 核心组件设计

#### 2.1 EinoMessageConverter - 消息转换器
**职责**: 在现有消息格式与 eino schema.Message 之间转换

```go
type EinoMessageConverter struct {
    core *core.Core
}

// 将 SessionContext 转换为 eino 消息
func (c *EinoMessageConverter) ConvertToEinoMessages(sessionContext *SessionContext) []*schema.Message

// 将 eino 消息转换回系统格式
func (c *EinoMessageConverter) ConvertFromEinoMessages(messages []*schema.Message) []*types.MessageContext
```

**转换映射关系**:
- `types.USER_ROLE_SYSTEM` ↔ `schema.System`
- `types.USER_ROLE_USER` ↔ `schema.User`
- `types.USER_ROLE_ASSISTANT` ↔ `schema.Assistant`
- `types.USER_ROLE_TOOL` ↔ `schema.Tool`

#### 2.2 EinoAgentFactory - Agent 工厂
**职责**: 创建和配置 eino ReAct Agent

```go
type EinoAgentFactory struct {
    core *core.Core
}

func (f *EinoAgentFactory) CreateReActAgent(ctx context.Context, adapter *ai.EinoAdapter) (*react.Agent, error)
func (f *EinoAgentFactory) createTools(ctx context.Context) ([]tool.BaseTool, error)
```

**支持的工具**:
- DuckDuckGo 搜索工具
- 现有的知识库搜索工具（需要适配）
- 未来扩展的其他工具

#### 2.3 EinoStreamHandler - 流式处理器
**职责**: 处理 eino Agent 的流式响应，适配现有的 ReceiveFunc

```go
type EinoStreamHandler struct {
    receiveFunc types.ReceiveFunc
    adapter     *ai.EinoAdapter
}

func (h *EinoStreamHandler) HandleStreamResponse(ctx context.Context, respChan <-chan *agent.ComposeResult) error
```

#### 2.4 ToolCallPersister - 工具调用持久化器
**职责**: 将工具调用过程保存到数据库作为聊天记录

```go
type ToolCallPersister struct {
    core      *core.Core
    sessionID string
    spaceID   string
    userID    string
}

// 保存工具调用开始记录
func (p *ToolCallPersister) SaveToolCallStart(ctx context.Context, toolName string, args interface{}) (string, error)

// 更新工具调用完成记录
func (p *ToolCallPersister) SaveToolCallComplete(ctx context.Context, toolCallMsgID string, result interface{}, success bool) error

// 创建工具调用消息格式
func (p *ToolCallPersister) createToolCallMessage(toolName string, args, result interface{}, status string) *types.ChatMessage
```

**工具调用记录格式**:
```json
{
  "role": "tool",
  "message": "🔧 工具调用: SearchUserKnowledges\n参数: {\"query\":\"golang新特性\"}\n结果: 找到5条相关知识",
  "msg_type": 1,
  "complete": 1
}
```

### 3. RequestAssistant 方法重构

#### 3.1 整体流程设计
```
用户请求 → 生成会话上下文 → 转换为 eino 消息 → 创建 ReAct Agent → 流式处理 → 工具调用记录 → 工具调用持久化 → 完成响应
```

**新增的工具调用持久化流程**:
```
工具调用开始 → 保存 role=tool 记录到数据库 → 推送 ToolTips → 执行工具 → 更新数据库记录 → 推送完成状态
```

#### 3.2 详细步骤

1. **上下文准备**
   - 复用现有的 `GenSessionContext` 方法
   - 构建 RAG 提示词
   - 处理多媒体附件

2. **消息转换**
   - 使用 `EinoMessageConverter` 转换消息格式
   - 保持现有的角色和内容结构

3. **Agent 创建**
   - 通过 `EinoAgentFactory` 创建 ReAct Agent
   - 配置模型参数（从现有配置读取）
   - 集成工具列表

4. **工具调用记录与持久化**
   - 使用之前创建的 `EinoAdapter` 进行实时记录
   - 通过 MessageModifier 拦截工具调用
   - **新增**: 使用 `ToolCallPersister` 保存工具调用到数据库
   - 实时推送工具状态到 WebSocket
   - 工具调用完成后更新数据库记录

5. **流式处理**
   - 区分流式和非流式请求
   - 使用 `EinoStreamHandler` 处理响应
   - 保持与现有 `ReceiveFunc` 的兼容性

### 4. 工具系统集成

#### 4.1 现有工具适配
- **知识库搜索**: 将 `rag.FunctionDefine` 适配为 eino 工具
- **DuckDuckGo 搜索**: 直接使用 eino-ext 提供的工具
- **其他工具**: 根据需要逐步迁移

#### 4.2 工具调用记录与持久化
- **实时记录**: 复用现有的 `ToolTips` 系统，通过 `EinoAdapter` 记录工具调用过程
- **持久化存储**: 新增 `ToolCallPersister` 将工具调用保存为 `role=tool` 的聊天记录
- **双轨制设计**: 
  - WebSocket 推送用于实时展示（保持现有机制）
  - 数据库存储用于历史查看和页面刷新后的展示
- **记录内容**: 包含工具名称、参数、执行结果、状态等完整信息

### 5. 错误处理和兼容性

#### 5.1 错误处理策略
- 保持与 `NormalAssistant` 相同的错误处理逻辑
- 使用 `handleAndNotifyAssistantFailed` 统一处理失败情况
- eino 特有错误的适配和转换

#### 5.2 向后兼容性
- 所有公共接口保持不变
- 内部实现完全替换为 eino 框架
- 配置参数复用现有系统

## 📝 实施计划

### 阶段 1: 基础组件实现
- [ ] `AutoAssistant` 结构体定义
- [ ] `EinoMessageConverter` 消息转换器
- [ ] `EinoAgentFactory` Agent 工厂
- [ ] `EinoStreamHandler` 流式处理器
- [ ] `ToolCallPersister` 工具调用持久化器

### 阶段 2: 核心方法重构
- [ ] `RequestAssistant` 方法实现
- [ ] 流式和非流式处理逻辑
- [ ] 错误处理和状态管理

### 阶段 3: 工具系统集成
- [ ] 现有工具的 eino 适配
- [ ] 工具调用实时记录集成
- [ ] 工具调用持久化实现
- [ ] WebSocket 推送保持
- [ ] 前端工具调用历史展示适配

### 阶段 4: 测试和验证
- [ ] 单元测试用例
- [ ] 集成测试验证
- [ ] 性能对比测试

## 🔧 技术细节

### 依赖项
```go
import (
    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/flow/agent"
    "github.com/cloudwego/eino/flow/agent/react"
    "github.com/cloudwego/eino/schema"
)
```

### 配置复用
- AI 模型配置: `core.Cfg().AI`
- 提示词配置: `core.Prompt()`
- 数据库和存储: 复用现有 `core.Store()`

### 性能考虑
- 消息转换的性能开销
- eino Agent 创建的资源消耗
- 流式处理的内存管理

## 🚀 预期收益

### 功能增强
- 更强大的工具调用能力
- 更好的 Agent 推理逻辑
- 更丰富的工具生态
- **工具调用历史可见**: 前端刷新后仍可查看完整的工具调用记录
- **完整对话记录**: 包含用户消息、AI回复、工具调用的完整对话链路

### 代码质量
- 更清晰的架构分离
- 更好的可测试性
- 更易于扩展和维护

### 系统性能
- 框架级别的优化
- 更好的并发处理
- 更稳定的工具调用

## 📋 验收标准

### 功能完整性
- [ ] 所有现有功能正常工作
- [ ] 工具调用实时记录完整（ToolTips 推送）
- [ ] 工具调用持久化正常（数据库存储）
- [ ] 前端刷新后工具调用历史可见
- [ ] 流式响应正常
- [ ] 错误处理正确

### 性能要求
- [ ] 响应时间不劣化
- [ ] 内存使用合理
- [ ] 并发处理能力保持

### 兼容性检查
- [ ] 接口完全兼容
- [ ] 配置无需修改
- [ ] 数据格式一致

## 🤔 需要确认的问题与实现分析

### 问题分析与实现建议

#### 1. **eino 版本选择**
**问题**: 当前使用的 eino 版本是否支持所需的所有功能？
**分析**: 基于现有的 `eino_test.go` 文件，项目已经在使用 eino 框架
**实现建议**: ✅ **可行**
- 当前版本支持 ReAct Agent、工具调用、流式处理
- 已验证可以正常工作，无版本兼容性问题

#### 2. **工具迁移策略**
**问题**: 现有的 `rag.FunctionDefine` 如何最佳地适配到 eino？
**实现建议**: ✅ **可行**
```go
// 现有: rag.FunctionDefine ([]openai.Tool)
// 目标: eino tool.BaseTool

// 实现适配器
func convertRAGToolsToEino(ragTools []openai.Tool) []tool.BaseTool {
    // 将 openai.Tool 封装为 eino tool.BaseTool
}
```

#### 3. **工具调用记录格式**
**问题**: 工具调用记录的消息格式是否需要特殊的结构化设计？前端如何区分和展示不同类型的工具调用记录？
**实现建议**: ✅ **可行**
```go
// 建议的消息结构
type ToolCallMessage struct {
    ToolName   string      `json:"tool_name"`
    Arguments  interface{} `json:"arguments"`
    Result     interface{} `json:"result,omitempty"`
    Status     string      `json:"status"` // "running", "success", "failed"
    StartTime  int64       `json:"start_time"`
    EndTime    int64       `json:"end_time,omitempty"`
}

// 序列化为 message 字段
message := fmt.Sprintf("🔧 %s", marshalToolCall(toolCall))
```

#### 4. **持久化策略**
**问题**: 工具调用记录是否需要单独的表，还是使用现有的 chat_message 表？如何处理长时间运行的工具调用？
**实现建议**: ✅ **推荐使用现有表**
- **优势**: 保持数据一致性，前端无需额外适配
- **实现**: 使用 `role=tool`, `msg_type=MESSAGE_TYPE_TOOL_TIPS`
- **长时间运行处理**: 先创建记录 `complete=MESSAGE_PROGRESS_GENERATING`，完成后更新状态

#### 5. **性能基准**
**问题**: 是否需要设定具体的性能基准要求？
**实现建议**: ✅ **建议设定基准**
- 响应时间: 不超过现有实现的 110%
- 内存使用: 控制消息转换开销
- 工具调用记录: 异步保存，不阻塞主流程

#### 6. **回滚策略**
**问题**: 如果出现问题，如何快速回滚到原有实现？
**实现建议**: ✅ **完全可行**
- 通过配置开关控制使用 `NormalAssistant` 还是 `AutoAssistant`
- 接口完全兼容，可以无缝切换
- 数据库结构无变化，回滚无风险

#### 7. **测试覆盖**
**问题**: 需要哪些特定的测试场景来验证改造效果？
**实现建议**: ✅ **可实现**
需要测试场景：
- 工具调用记录的完整性
- 前端刷新后历史可见性
- 错误情况下的记录状态
- 并发工具调用的处理

#### 8. **前端适配**
**问题**: 前端是否需要特殊的逻辑来展示 `role=tool` 的聊天记录？
**实现建议**: ✅ **可行，需要协调**
```javascript
// 前端渲染逻辑
function renderMessage(message) {
    switch(message.role) {
        case 'user': return renderUserMessage(message);
        case 'assistant': return renderAssistantMessage(message);
        case 'tool': return renderToolCallMessage(message); // 新增
    }
}
```

### 🎯 实现可行性总结

| 问题类别 | 可行性 | 风险等级 | 建议 |
|---------|--------|----------|------|
| eino 版本 | ✅ 可行 | 低 | 直接使用现有版本 |
| 工具迁移 | ✅ 可行 | 中 | 创建适配层，逐步迁移 |
| 记录格式 | ✅ 可行 | 低 | 使用 JSON 结构化格式 |
| 持久化策略 | ✅ 可行 | 低 | 复用现有 chat_message 表 |
| 性能基准 | ✅ 可行 | 中 | 需要压测验证 |
| 回滚策略 | ✅ 可行 | 低 | 配置开关控制 |
| 测试覆盖 | ✅ 可行 | 中 | 需要完善测试用例 |
| 前端适配 | ✅ 可行 | 中 | 需要前端开发配合 |

### 📋 具体实现建议

#### 最小风险方案
1. **使用现有 chat_message 表**: 避免数据库变更风险
2. **保持接口兼容性**: 完全不影响现有代码
3. **配置开关控制**: 支持快速回滚
4. **异步持久化**: 不影响响应性能

#### 关键实现点
1. **工具调用状态管理**: `running → success/failed`
2. **消息格式标准化**: JSON 结构 + 用户友好展示
3. **错误处理**: 工具调用失败时的记录更新
4. **并发安全**: 多个工具同时调用的记录处理

**结论**: 所有问题都有可行的实现方案，风险可控，建议按计划推进！

---

**下一步**: 等待 review 确认后，按照计划逐步实施各个组件的开发。  
reviewer: 确认通过