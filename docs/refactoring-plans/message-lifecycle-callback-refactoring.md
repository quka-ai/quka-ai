# 基于 Eino Callback 的消息生命周期重构计划

## 问题描述和背景

当前的消息生命周期管理机制存在以下问题：

1. **消息初始化时机不够统一**：初始化消息到数据库的操作在 `RequestAssistant` 方法中通过 `InitAssistantMessage` 调用（auto_assistant.go:56-58），不够灵活
2. **工具调用记录不够完整**：虽然有 `ToolCallPersister` 机制，但与主消息流的集成不够紧密
3. **多轮对话状态管理复杂**：当 AI 自动生成工具调用或进行多轮对话时，状态管理分散在不同地方
4. **eino callback 机制未充分利用**：现有的 eino callback 系统（`NewCallbackHandlers`）主要用于记录 token 使用情况和调用 `handler.doneFunc`，未用于完整的消息生命周期管理

## 改造目标

基于 **Eino Framework Callback 机制**将消息生命周期管理（创建、更新、完成）完全迁移到 eino 的 callback 生命周期中：

1. **eino OnStart Callback**：在 Agent/ChatModel 开始执行时自动创建 assistant 消息记录到数据库
2. **eino OnEnd Callback**：在执行结束时自动更新消息状态为完成 (替代现有的 handler.doneFunc)
3. **eino Tool Callback 集成**：工具调用的开始和结束也通过 eino callback 机制管理
4. **统一的 eino 状态管理**：不管是简单对话还是复杂的多轮工具调用，都通过 eino 的统一 callback 机制处理

## 详细实施方案

### 阶段一：基于 Eino Callback 的消息生命周期管理器

#### 1.1 创建 Eino Callback 消息生命周期管理器
```go
// EinoMessageLifecycleCallback Eino 消息生命周期回调管理器
type EinoMessageLifecycleCallback struct {
    callbacks.HandlerBuilder // 嵌入 eino HandlerBuilder
    
    core           *core.Core
    userReqMessage *types.ChatMessage
    aiMessage      *types.ChatMessage    // 代表一次完整的 AI 响应会话，在 Agent OnStart 中创建
    ext            types.ChatMessageExt
    receiver       types.Receiver
    receiveFunc    types.ReceiveFunc     // 保存 receiveFunc，用于响应处理
    doneFunc       types.DoneFunc        // 保存 doneFunc，在 OnEnd 中调用
    mutex          sync.RWMutex
}

// 消息生命周期范围说明:
// - aiMessage 代表一次完整的 RequestAssistant() 调用产生的 AI 响应
// - aiMessage 的内容是基于工具调用结果生成的最终回答，但不直接包含工具调用过程
// - Agent OnStart -> 创建 aiMessage (MESSAGE_PROGRESS_GENERATING)
// - Agent OnEnd -> 完成 aiMessage (MESSAGE_PROGRESS_COMPLETE)
// - 工具调用会创建独立的工具消息记录，用于展示执行过程，但最终结果体现在 aiMessage 中
```

#### 1.2 实现 Eino OnStart Callback - 消息初始化
```go
func (c *EinoMessageLifecycleCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
    // 只在 Agent 组件开始时创建消息（Agent 级别代表整个对话会话）
    if !c.isAgentComponent(info) {
        return ctx
    }
    
    // 创建 assistant 消息记录，代表一次完整的 AI 响应会话
    // (替代原来在 RequestAssistant 中的 InitAssistantMessage 调用)
    msgID := utils.GenUniqIDStr()
    
    // 获取正确的消息序号（在会话中的顺序序号）
    seqID, err := c.core.GetChatMessageSequence(ctx, c.userReqMessage.SpaceID, c.userReqMessage.SessionID)
    if err != nil {
        slog.Error("failed to get chat message sequence in Agent OnStart callback", slog.Any("error", err))
        return ctx
    }
    
    aiMessage, err := initAssistantMessage(ctx, c.core, msgID, seqID, c.userReqMessage, c.ext)
    if err != nil {
        slog.Error("failed to init assistant message in Agent OnStart callback", slog.Any("error", err))
        return ctx
    }
    
    c.mutex.Lock()
    c.aiMessage = aiMessage
    c.mutex.Unlock()
    
    // 🔥 获取并保存 receiveFunc 和 doneFunc，供后续使用
    c.receiveFunc = c.receiver.GetReceiveFunc()
    c.doneFunc = c.receiver.GetDoneFunc(func(msg *types.ChatMessage) {
        // 执行原有的 callback 逻辑，如知识库关联
    })
    
    // 发送初始化消息通知
    if err := c.receiver.PublishMessage(types.WS_EVENT_ASSISTANT_INIT, &types.StreamMessage{
        MessageID: aiMessage.ID,
        SessionID: aiMessage.SessionID,
        Message:   "",
        Complete:  int32(aiMessage.Complete),
        MsgType:   aiMessage.MsgType,
        StartAt:   0,
    }); err != nil {
        slog.Error("failed to publish message init", slog.Any("error", err))
    }
    
    slog.Debug("AI message session created", 
        slog.String("msg_id", msgID), 
        slog.String("session_id", c.userReqMessage.SessionID))
    
    return ctx
}

// isAgentComponent 检查是否为 Agent 组件
func (c *EinoMessageLifecycleCallback) isAgentComponent(info *callbacks.RunInfo) bool {
    // eino ReAct Agent 的组件名通常是 "react.Agent"
    return info.Name == "react.Agent" || info.Type == "agent"
}

// NewEinoMessageLifecycleCallback 创建 Eino 消息生命周期回调管理器
func NewEinoMessageLifecycleCallback(core *core.Core, userReqMsg *types.ChatMessage, ext types.ChatMessageExt, receiver types.Receiver) *EinoMessageLifecycleCallback {
    return &EinoMessageLifecycleCallback{
        core:           core,
        userReqMessage: userReqMsg,
        ext:            ext,
        receiver:       receiver,
    }
}
```

#### 1.3 实现 Eino OnEnd Callback - 消息完成
```go
func (c *EinoMessageLifecycleCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
    // 只在 Agent 组件结束时完成消息（整个对话会话结束）
    if !c.isAgentComponent(info) {
        return ctx
    }
    
    c.mutex.RLock()
    aiMessage := c.aiMessage
    c.mutex.RUnlock()
    
    if aiMessage == nil {
        slog.Warn("aiMessage is nil in Agent OnEnd callback, session may not have been properly initialized")
        return ctx
    }
    
    // 更新消息状态为完成 (替代原来的 handler.doneFunc)
    // 此时整个 AI 响应会话已完成，包括所有工具调用和最终回答
    if err := c.core.Store().ChatMessageStore().UpdateMessageCompleteStatus(
        ctx, aiMessage.SessionID, aiMessage.ID, int32(types.MESSAGE_PROGRESS_COMPLETE)); err != nil {
        slog.Error("failed to update message complete status in Agent OnEnd callback", slog.Any("error", err))
        return ctx
    }
    
    // 🔥 直接调用已保存的 doneFunc（成功场景）
    if c.doneFunc != nil {
        c.doneFunc(nil)
    }
    
    slog.Debug("AI message session completed", 
        slog.String("msg_id", aiMessage.ID), 
        slog.String("session_id", aiMessage.SessionID))
    
    return ctx
}
```

#### 1.4 实现 Eino OnError Callback - 错误处理
```go
func (c *EinoMessageLifecycleCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
    // 只在 Agent 组件出错时处理消息失败（整个对话会话失败）
    if !c.isAgentComponent(info) {
        return ctx
    }
    
    c.mutex.RLock()
    aiMessage := c.aiMessage
    c.mutex.RUnlock()
    
    if aiMessage != nil {
        // 更新消息状态为失败
        if updateErr := c.core.Store().ChatMessageStore().UpdateMessageCompleteStatus(
            ctx, aiMessage.SessionID, aiMessage.ID, int32(types.MESSAGE_PROGRESS_FAILED)); updateErr != nil {
            slog.Error("failed to update message failed status in Agent OnError callback", 
                slog.Any("original_error", err),
                slog.Any("update_error", updateErr))
        }
        
        slog.Error("AI message session failed", 
            slog.String("msg_id", aiMessage.ID), 
            slog.String("session_id", aiMessage.SessionID),
            slog.Any("error", err))
    }
    
    // 🔥 直接调用已保存的 doneFunc（错误场景）
    if c.doneFunc != nil {
        c.doneFunc(err)
    }
    
    return ctx
}
```

### 阶段二：Eino Tool Callback 集成

#### 2.1 增强工具调用的 Eino Callback 机制
```go
// EinoToolLifecycleCallback 工具调用生命周期回调管理器
type EinoToolLifecycleCallback struct {
    callbacks.HandlerBuilder
    
    persister       *ToolCallPersister
    parentMessage   *types.ChatMessage
    activeToolCalls map[string]*ToolCallState // tool_id -> state
    mutex           sync.RWMutex
}

// 在工具开始执行时创建记录
func (c *EinoToolLifecycleCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
    if !c.isToolComponent(info) {
        return ctx
    }
    
    toolName := info.Name
    toolID := c.generateToolID(info)
    
    // 创建工具调用记录
    msgID, err := c.persister.SaveToolCallStart(ctx, toolName, input)
    if err != nil {
        slog.Error("failed to save tool call start", slog.Any("error", err))
        return ctx
    }
    
    c.mutex.Lock()
    c.activeToolCalls[toolID] = &ToolCallState{
        MessageID: msgID,
        ToolName:  toolName,
        StartTime: time.Now(),
    }
    c.mutex.Unlock()
    
    return ctx
}

// 在工具执行完成时更新记录
func (c *EinoToolLifecycleCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
    if !c.isToolComponent(info) {
        return ctx
    }
    
    toolID := c.generateToolID(info)
    
    c.mutex.Lock()
    toolState := c.activeToolCalls[toolID]
    delete(c.activeToolCalls, toolID)
    c.mutex.Unlock()
    
    if toolState != nil {
        // 更新工具调用完成记录
        if err := c.persister.SaveToolCallComplete(ctx, toolState.MessageID, output, true); err != nil {
            slog.Error("failed to save tool call complete", slog.Any("error", err))
        }
    }
    
    return ctx
}
```

### 阶段三：重构 AutoAssistant 集成

#### 3.1 移除直接的消息初始化调用
```go
// 修改 RequestAssistant 方法
func (a *AutoAssistant) RequestAssistant(ctx context.Context, reqMsg *types.ChatMessage, receiver types.Receiver, aiCallOptions *types.AICallOptions) error {
    // ... 现有逻辑 ...
    
    // ❌ 移除这部分直接调用
    // aiMessage, err := a.InitAssistantMessage(ctx, msgID, seqID, reqMsg, ext)
    
    // ✅ 创建带消息生命周期管理的 callback
    lifecycleCallback := NewEinoMessageLifecycleCallback(a.core, reqMsg, ext, receiver)
    toolCallback := NewEinoToolLifecycleCallback(persister, reqMsg)
    
    // ✅ 使用增强的 callback handler
    callbackHandler := NewEnhancedEinoCallbackHandlers(
        modelConfig.ModelName, 
        reqMsg.ID,
        lifecycleCallback,
        toolCallback,
        responseHandler,
    )
    
    // ... 其余逻辑不变 ...
}
```

#### 3.2 增强的 Callback Handler 创建
```go
func NewEnhancedEinoCallbackHandlers(
    modelName, reqMessageID string,
    lifecycleCallback *EinoMessageLifecycleCallback,
    toolCallback *EinoToolLifecycleCallback,
    responseHandler *EinoResponseHandler,
) callbacks.Handler {
    
    return callbackhelper.NewHandlerHelper().
        // ChatModel 回调 - 负责消息生命周期和 token 统计
        ChatModel(&callbackhelper.ModelCallbackHandler{
            OnStart: lifecycleCallback.OnStart, // 🔥 消息初始化
            OnEnd: func(ctx context.Context, runInfo *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
                // Token 使用统计
                res := model.ConvCallbackOutput(output)
                if res.TokenUsage != nil {
                    go process.NewRecordChatUsageRequest(modelName, types.USAGE_SUB_TYPE_CHAT, reqMessageID, &goopenai.Usage{
                        TotalTokens:      res.TokenUsage.TotalTokens,
                        PromptTokens:     res.TokenUsage.PromptTokens,
                        CompletionTokens: res.TokenUsage.CompletionTokens,
                    })
                }
                
                // 🔥 消息完成处理
                return lifecycleCallback.OnEnd(ctx, runInfo, output)
            },
            OnError: lifecycleCallback.OnError, // 🔥 消息错误处理
            OnEndWithStreamOutput: func(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
                // 流式输出处理
                go safe.Run(func() {
                    ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
                    defer cancel()
                    if err := responseHandler.HandleStreamResponse(ctx, output); err != nil {
                        slog.Error("failed to handle stream response", slog.Any("error", err))
                        return
                    }
                    // 流式处理完成后，也要调用消息完成逻辑
                    lifecycleCallback.OnEnd(ctx, runInfo, nil)
                })
                return ctx
            },
        }).
        // Tool 回调 - 负责工具调用生命周期
        Tool(&callbackhelper.ToolCallbackHandler{
            OnStart: toolCallback.OnStart, // 🔥 工具调用开始
            OnEnd:   toolCallback.OnEnd,   // 🔥 工具调用完成
            OnError: toolCallback.OnError, // 🔥 工具调用错误
        }).
        Handler()
}
```

## 关键考虑点

### Eino Callback 组件过滤
- 需要正确识别哪些组件需要触发消息生命周期事件
- Agent 组件: `info.Name == "react.Agent"` 或 `info.Type == "agent"`
- ChatModel 组件: `info.Type == "model"` 
- Tool 组件: `info.Type == "tool"`

### 数据一致性
- 使用数据库事务确保消息创建和状态更新的原子性
- 在 eino callback 中实现重试机制处理数据库操作失败的情况
- 确保 OnStart 创建的消息记录在 OnEnd 中能正确找到

### 并发安全
- 多个工具可能并发调用，需要确保 `activeToolCalls` map 的线程安全
- 使用互斥锁保护 `aiMessage` 字段的读写
- eino callback 本身可能并发执行，需要考虑竞态条件

### Eino Callback 错误处理
- eino callback 执行失败不应影响主流程的 AI 对话
- 记录详细的错误日志便于排查 callback 问题
- 实现降级机制：callback 失败时仍然要确保消息状态正确

### 性能影响
- eino callback 执行应该尽量异步，避免阻塞 AI 响应
- 考虑批量更新机制减少数据库操作频次
- 避免在 callback 中执行耗时操作

### 向后兼容性
- 保留原有的 `InitAssistantMessage` 方法作为备用
- 确保与 `NormalAssistant` 的接口兼容性
- 支持渐进式迁移，可以逐步启用新的 callback 机制

## 实施时间线

### 第一周：Eino Callback 基础设施搭建
- [ ] 创建 `EinoMessageLifecycleCallback` 结构体和基础方法
- [ ] 实现 eino OnStart、OnEnd、OnError callback 的基础框架
- [ ] 编写单元测试验证 eino callback 功能

### 第二周：Eino Tool Callback 集成
- [ ] 创建 `EinoToolLifecycleCallback` 管理器
- [ ] 实现工具调用的 eino callback 机制
- [ ] 集成到现有的 `NotifyingTool` 系统，替代内部通知机制

### 第三周：AutoAssistant 重构
- [ ] 重构 `RequestAssistant` 方法，移除直接 `InitAssistantMessage` 调用
- [ ] 实现 `NewEnhancedEinoCallbackHandlers` 函数替代原有的 `NewCallbackHandlers`
- [ ] 进行端到端测试，确保流式和非流式响应都正常工作

## 状态追踪

- [ ] **阶段一：基于 Eino Callback 的消息生命周期管理器** - 待开始
- [ ] **阶段二：Eino Tool Callback 集成** - 待开始  
- [ ] **阶段三：重构 AutoAssistant 集成** - 待开始

## 需要确认的问题

1. **Eino Callback 组件识别**：如何准确识别 Agent、ChatModel、Tool 组件，避免重复触发？
2. **消息 ID 生成策略**：在 OnStart callback 中生成的消息 ID 如何与现有的 msgID、seqID 协调？
3. **流式响应处理**：在 `OnEndWithStreamOutput` 中如何正确触发消息完成逻辑？
4. **错误恢复机制**：如果 eino callback 执行失败，是否需要实现自动重试或降级？
5. **工具并发执行**：多个工具同时调用时，如何确保每个工具的生命周期记录都正确？
6. **向后兼容性**：是否需要保持与现有 `NormalAssistant` 的兼容性？

## 相关文件列表

### 需要修改的文件
- `app/logic/v1/auto_assistant.go` - 主要重构目标，移除直接消息初始化，集成 eino callback
- `app/logic/v1/ai.go` - 保留 `initAssistantMessage` 函数供 callback 使用
- 可能需要查看 `pkg/types/receiver.go` - 确认 Receiver 接口是否需要调整

### 需要创建的文件
- `app/logic/v1/eino_message_lifecycle_callback.go` - Eino 消息生命周期回调管理器
- `app/logic/v1/eino_tool_lifecycle_callback.go` - Eino 工具调用生命周期回调管理器
- `app/logic/v1/enhanced_eino_callback_handlers.go` - 增强的 Eino 回调处理器

### 测试文件
- `app/logic/v1/eino_message_lifecycle_callback_test.go`
- `app/logic/v1/eino_tool_lifecycle_callback_test.go`
- `app/logic/v1/auto_assistant_eino_integration_test.go`

### 需要参考的现有文件
- `app/logic/v1/auto_assistant_logger.go` - 学习 eino callback 的实现模式
- `app/logic/v1/auto_assistant.go:1052-1096` - 当前的 `NewCallbackHandlers` 实现

## 核心优势

基于 **Eino Framework** 的 callback 机制重构将带来：

1. **统一性**：所有消息和工具生命周期事件都通过 eino 原生 callback 处理
2. **完整性**：AI 对话和工具调用的每个步骤都有数据库记录，支持多轮对话场景
3. **原生集成**：充分利用 eino 框架的 callback 系统，减少自定义封装
4. **数据一致性**：通过 eino 的统一生命周期确保状态管理的可靠性
5. **可扩展性**：未来添加新的生命周期事件更容易，直接扩展 eino callback

## 备注

这个基于 Eino Callback 的重构将使消息生命周期管理更加原生化和统一化，充分利用 eino 框架的能力。实施过程中需要特别注意 eino callback 的组件识别和并发处理。