# Vision 和 OCR Tool 集成改造计划

## 1. 问题描述

### 当前实现
- 用户发送带图片/PDF的消息时，系统直接将内容转换为 Vision Model 的请求格式
- 直接调用 Vision Model 处理所有包含图片的请求
- 没有利用 LLM 的语义理解能力来选择合适的工具

### 存在的问题
1. **缺乏智能判断**:用户可能只是想提取图片中的文字(OCR场景),但系统仍然调用 Vision Model 进行完整的图像理解
2. **资源浪费**:Vision Model 通常比 OCR 更耗资源,对于简单的文字提取场景是过度使用
3. **工具未被充分利用**:已经实现了 OCR Tool 和 Vision Tool,但它们没有被集成到 Chat 流程中

### 改造目标
让 Chat Model (LLM) 根据用户的语义自动选择调用 OCR Tool 还是 Vision Tool:
- **OCR Tool**: 当用户需要提取、识别图片/PDF中的文字时
- **Vision Tool**: 当用户需要理解图片内容、场景、物体等视觉信息时

## 2. 技术方案

### 2.1 核心思路
将图片处理从"直接调用 Vision Model"改为"通过 Tool Calling 机制让 LLM 选择合适的工具"

### 2.2 实现步骤

#### Step 1: 移除直接使用 Vision Model 的逻辑
**文件**: `app/logic/v1/auto_assistant.go`

**当前代码** (第 920-926 行):
```go
// 检查消息中是否包含多媒体内容,决定使用哪种模型
if f.containsMultimediaContent(config.Messages) {
    chatModel = config.Core.Srv().AI().GetVisionAI()
} else {
    chatModel = config.Core.Srv().AI().GetChatAI(config.EnableThinking)
}
```

**改造方案**:
```go
// 始终使用 Chat Model,不再根据是否包含多媒体内容切换模型
chatModel = config.Core.Srv().AI().GetChatAI(config.EnableThinking)
```

**理由**:
- Chat Model 具备 Tool Calling 能力,可以根据语义选择工具
- Vision Model 不再被直接调用,而是作为 Vision Tool 的底层实现

#### Step 2: 将 OCR 和 Vision 工具添加到 Agent 的工具列表
**文件**: `app/logic/v1/auto_assistant.go`

**当前代码** (第 462-483 行):
```go
func (f *EinoAgentFactory) CreateAutoRagReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message) (*react.Agent, *types.ModelConfig, error) {
    config := NewAgentConfig(agentCtx, toolWrapper, f.core, agentCtx.EnableThinking, messages)

    // 应用选项
    options := []AgentOption{
        &WithWebSearch{
            Enable: agentCtx.EnableWebSearch,
        }, // 支持网络搜索
        &WithRAG{
            Enable: agentCtx.EnableKnowledge,
        }, // 支持知识库搜索
        NewWithKnowledgeTools(true), // 支持知识库 CRUD 工具
    }

    for _, option := range options {
        if err := option.Apply(config); err != nil {
            slog.Warn("Failed to apply butler agent option", slog.Any("error", err))
        }
    }

    return f.CreateReActAgentWithConfig(config)
}
```

**改造方案**:
```go
func (f *EinoAgentFactory) CreateAutoRagReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message) (*react.Agent, *types.ModelConfig, error) {
    config := NewAgentConfig(agentCtx, toolWrapper, f.core, agentCtx.EnableThinking, messages)

    // 检查消息中是否包含多媒体内容
    hasMultimedia := f.containsMultimediaContent(messages)

    // 应用选项
    options := []AgentOption{
        &WithWebSearch{
            Enable: agentCtx.EnableWebSearch,
        }, // 支持网络搜索
        &WithRAG{
            Enable: agentCtx.EnableKnowledge,
        }, // 支持知识库搜索
        NewWithKnowledgeTools(true),        // 支持知识库 CRUD 工具
        NewWithOCRTool(hasMultimedia),      // 当有多媒体内容时启用 OCR 工具
        NewWithVisionTool(hasMultimedia),   // 当有多媒体内容时启用 Vision 工具
    }

    for _, option := range options {
        if err := option.Apply(config); err != nil {
            slog.Warn("Failed to apply butler agent option", slog.Any("error", err))
        }
    }

    return f.CreateReActAgentWithConfig(config)
}
```

**理由**:
- 只有当消息中包含多媒体内容时才启用 OCR 和 Vision 工具
- 避免不必要的工具加载,减少 LLM 的 token 消耗

#### Step 3: 修改图片附件处理逻辑
**文件**: `pkg/types/chat_message.go` 或相关的消息处理文件

**当前逻辑**:
- 用户上传图片后,图片 URL 被添加到消息的 `MultiContent` 中
- 消息被转换为 Vision Model 的格式并直接调用

**改造方案**:
有两种选择:

**方案 A: 保留 MultiContent,让工具主动提取图片 URL**
```go
// 消息仍然包含 MultiContent
message := &schema.Message{
    Role: schema.User,
    Content: "这张图片是什么?",
    MultiContent: []schema.ChatMessagePart{
        {Type: schema.ChatMessagePartTypeText, Text: "这张图片是什么?"},
        {Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://..."}},
    },
}

// LLM 看到消息中有图片,根据用户问题选择调用 OCR 或 Vision Tool
// Tool 从消息的 MultiContent 中提取图片 URL
```

**方案 B: 不在消息中直接包含图片,而是提示 LLM 有图片可用**
```go
// 消息不包含图片,而是包含图片的引用信息
message := &schema.Message{
    Role: schema.User,
    Content: "这张图片是什么? [图片 URL: https://...]",
}

// LLM 看到有图片 URL,根据用户问题选择调用工具
// Tool 接收图片 URL 作为参数
```

**推荐使用方案 A**,因为:
- 符合多模态消息的标准格式
- Vision Tool 和 OCR Tool 已经支持接收图片 URL 作为参数
- 但需要修改工具,让它们能够从上下文中提取图片 URL

#### Step 4: 修改 OCR 和 Vision Tool 的描述和参数
**文件**:
- `pkg/ai/tools/ocr/ocr_tool.go`
- `pkg/ai/tools/vision/vision_tool.go`

**OCR Tool 当前描述**:
```go
Name: "ocr",
Desc: "从图像中提取文字内容。当用户提到需要识别图片中的文字、扫描文档、读取图片内容时使用此工具。支持 PDF 和常见图片格式(PNG、JPEG、GIF、WEBP、BMP)。支持单个或批量处理多个图片",
```

**Vision Tool 当前描述**:
```go
Name: "vision",
Desc: "理解和分析图像内容。当用户需要了解图片中的场景、物体、人物、活动等视觉信息时使用此工具。此工具可以描述图片内容、回答关于图片的问题、识别图片中的元素。支持单个或批量处理多个图片",
```

**改造建议**:
描述已经很清晰,无需修改。但需要考虑:
- 两个工具都要求 `image_urls` 参数
- 用户消息中的图片 URL 如何传递给工具?

**解决方案**:
1. **让工具支持从上下文中自动提取图片 URL**:
   - 如果 `image_urls` 参数为空或不提供,工具自动从消息历史中提取
   - 这需要在工具中添加对消息历史的访问

2. **让 LLM 显式传递图片 URL**:
   - 在 System Prompt 中告诉 LLM:"如果消息中包含图片,调用工具时必须传递图片 URL"
   - LLM 从消息的 MultiContent 中提取 URL 并作为参数传递

**推荐方案 2**,因为:
- 符合 Tool Calling 的标准流程
- LLM 有足够的能力从 MultiContent 中提取 URL
- 工具实现保持简单,不需要访问消息历史

## 3. 详细实现步骤

### 3.1 修改 `CreateReActAgentWithConfig` 函数
**文件**: `app/logic/v1/auto_assistant.go` (第 909-963 行)

```go
func (f *EinoAgentFactory) CreateReActAgentWithConfig(config *AgentConfig) (*react.Agent, *types.ModelConfig, error) {
    var err error

    var chatModel types.ChatModel
    // 移除根据多媒体内容选择模型的逻辑
    // 始终使用 Chat Model
    if config.ModelOverride != nil {
        if chatModel, err = srv.SetupAIDriver(context.Background(), *config.ModelOverride); err != nil {
            return nil, nil, err
        }
    } else {
        // 始终使用 Chat Model (不再检查是否包含多媒体)
        chatModel = config.Core.Srv().AI().GetChatAI(config.EnableThinking)
    }

    // ... 其余代码保持不变
}
```

### 3.2 修改 `CreateAutoRagReActAgent` 函数
**文件**: `app/logic/v1/auto_assistant.go` (第 461-483 行)

```go
func (f *EinoAgentFactory) CreateAutoRagReActAgent(agentCtx *types.AgentContext, toolWrapper NotifyToolWrapper, messages []*schema.Message) (*react.Agent, *types.ModelConfig, error) {
    config := NewAgentConfig(agentCtx, toolWrapper, f.core, agentCtx.EnableThinking, messages)

    // 检查消息中是否包含多媒体内容
    hasMultimedia := f.containsMultimediaContent(messages)

    // 应用选项
    options := []AgentOption{
        &WithWebSearch{
            Enable: agentCtx.EnableWebSearch,
        },
        &WithRAG{
            Enable: agentCtx.EnableKnowledge,
        },
        NewWithKnowledgeTools(true),
        NewWithOCRTool(hasMultimedia),      // 新增
        NewWithVisionTool(hasMultimedia),   // 新增
    }

    for _, option := range options {
        if err := option.Apply(config); err != nil {
            slog.Warn("Failed to apply agent option", slog.Any("error", err))
        }
    }

    return f.CreateReActAgentWithConfig(config)
}
```

### 3.3 完善 System Prompt
**文件**: 可能在 `app/core/prompt_manager.go` 或相关的 Prompt 管理文件

在 System Prompt 中添加关于图片处理的指导:

```
当用户的消息中包含图片时:
1. 如果用户需要提取、识别、读取图片中的文字内容,请使用 OCR 工具
2. 如果用户需要理解图片的场景、物体、人物、活动等视觉信息,请使用 Vision 工具
3. 调用工具时,请从消息的图片 URL 中提取 URL 并作为参数传递
```

## 4. 测试计划

### 4.1 OCR 场景测试
**测试用例 1**: 用户上传一张包含文字的图片,问"这张图片上写的是什么?"
- **期望**: LLM 调用 OCR Tool
- **验证**: 检查工具调用日志,确认调用了 OCR Tool

**测试用例 2**: 用户上传扫描的 PDF,问"帮我提取这个文档的文字"
- **期望**: LLM 调用 OCR Tool
- **验证**: 检查工具调用日志,确认调用了 OCR Tool

### 4.2 Vision 场景测试
**测试用例 3**: 用户上传一张风景照,问"这是哪里?"
- **期望**: LLM 调用 Vision Tool
- **验证**: 检查工具调用日志,确认调用了 Vision Tool

**测试用例 4**: 用户上传一张照片,问"这张照片里有什么?"
- **期望**: LLM 调用 Vision Tool
- **验证**: 检查工具调用日志,确认调用了 Vision Tool

### 4.3 混合场景测试
**测试用例 5**: 用户上传一张海报图片,问"这张海报的标题是什么?"
- **期望**: LLM 可能调用 OCR Tool 或 Vision Tool (都可以完成任务)
- **验证**: 检查结果是否正确

**测试用例 6**: 用户上传多张图片,分别询问文字内容和场景信息
- **期望**: LLM 针对不同问题调用不同工具
- **验证**: 检查工具调用日志

## 5. 潜在问题和解决方案

### 5.1 LLM 是否能准确选择工具?
**问题**: LLM 可能无法准确区分 OCR 和 Vision 场景

**解决方案**:
1. 优化工具描述,让差异更明显
2. 在 System Prompt 中明确告知使用场景
3. 收集用户反馈,持续优化

### 5.2 图片 URL 传递问题
**问题**: LLM 是否能正确从 MultiContent 中提取图片 URL?

**解决方案**:
1. 测试主流 LLM (GPT-4, Claude, Qwen 等) 的能力
2. 如果 LLM 无法提取,考虑在消息中显式提示图片 URL
3. 或者修改工具实现,让工具自动从上下文中提取

### 5.3 性能问题
**问题**: 增加工具调用会增加延迟

**解决方案**:
1. 只有在消息包含多媒体时才启用工具
2. 优化工具实现,减少不必要的网络请求
3. 考虑缓存机制

### 5.4 成本问题
**问题**: 工具调用会增加 Token 消耗

**解决方案**:
1. 监控 Token 使用情况
2. 评估是否需要针对简单场景做优化
3. 考虑提供用户配置选项,让用户选择是否启用智能工具选择

## 6. 回退方案

如果改造后发现效果不佳,可以通过以下方式回退:

1. **快速回退**: 在 `CreateReActAgentWithConfig` 中恢复原来的逻辑
   ```go
   if f.containsMultimediaContent(config.Messages) {
       chatModel = config.Core.Srv().AI().GetVisionAI()
   } else {
       chatModel = config.Core.Srv().AI().GetChatAI(config.EnableThinking)
   }
   ```

2. **渐进式部署**: 添加功能开关,让用户选择使用新模式还是旧模式
   ```go
   if config.AgentCtx.EnableSmartToolSelection {
       // 使用新的工具选择机制
   } else {
       // 使用旧的直接调用 Vision Model 机制
   }
   ```

## 7. 实施记录

### Phase 1: 代码修改 ✅ (已完成 - 2025-12-31)
- [x] 修改 `CreateReActAgentWithConfig` 函数 - 移除根据多媒体内容选择模型的逻辑
- [x] 修改 `CreateAutoRagReActAgent` 函数 - 添加 OCR 和 Vision 工具
- [x] 修改 `ConvertFromChatMessages` 函数 - 将图片 URL 使用 markdown 语法显式包含在消息文本中

### 实际修改内容:

#### 1. `CreateReActAgentWithConfig` (auto_assistant.go:909-923)
**修改内容**: 始终使用 Chat Model，不再根据是否包含多媒体内容切换模型
```go
// 修改前: 根据是否有多媒体内容选择 Vision Model 或 Chat Model
if f.containsMultimediaContent(config.Messages) {
    chatModel = config.Core.Srv().AI().GetVisionAI()
} else {
    chatModel = config.Core.Srv().AI().GetChatAI(config.EnableThinking)
}

// 修改后: 始终使用 Chat Model
// 始终使用 Chat Model，不再根据是否包含多媒体内容切换模型
// 图片处理通过 OCR Tool 和 Vision Tool 来完成，由 LLM 根据语义选择合适的工具
chatModel = config.Core.Srv().AI().GetChatAI(config.EnableThinking)
```

#### 2. `CreateAutoRagReActAgent` (auto_assistant.go:461-488)
**修改内容**: 添加 OCR 和 Vision 工具选项
```go
// 新增: 检查消息中是否包含多媒体内容
hasMultimedia := f.containsMultimediaContent(messages)

// 新增: 添加 OCR 和 Vision 工具选项
options := []AgentOption{
    &WithWebSearch{Enable: agentCtx.EnableWebSearch},
    &WithRAG{Enable: agentCtx.EnableKnowledge},
    NewWithKnowledgeTools(true),
    NewWithOCRTool(hasMultimedia),      // 支持 OCR 图片文字提取工具
    NewWithVisionTool(hasMultimedia),   // 支持 Vision 图片理解工具
}
```

#### 3. `ConvertFromChatMessages` (auto_assistant.go:272-285)
**修改内容**: 使用 markdown 语法在消息文本中包含图片 URL
```go
// 修改前: 使用 MultiContent 传递图片
if len(msg.Attach) > 0 {
    multiContent := msg.Attach.ToMultiContent(msg.Message, c.core.FileStorage())
    einoMsg.MultiContent = c.convertToEinoMultiContent(multiContent)
}

// 修改后: 使用 markdown 语法在消息文本中包含图片 URL
if len(msg.Attach) > 0 {
    if einoMsg.Content != "" {
        einoMsg.Content += "\n\n"
    }
    for i, item := range msg.Attach {
        url := lo.If(item.SignURL != "", item.SignURL).Else(item.URL)
        // 使用 markdown 图片语法: ![alt](url)
        einoMsg.Content += fmt.Sprintf("![图片%d](%s)\n", i+1, url)
    }
}
```

#### 4. `containsMultimediaContent` (auto_assistant.go:647-661) ⚠️ 重要修复
**修改内容**: 适配方案 B，检查消息文本中的 markdown 图片语法
```go
// 修改前: 只检查 MultiContent
func (f *EinoAgentFactory) containsMultimediaContent(messages []*schema.Message) bool {
    for _, msg := range messages {
        if len(msg.MultiContent) > 0 {
            return true
        }
    }
    return false
}

// 修改后: 检查 markdown 图片语法
func (f *EinoAgentFactory) containsMultimediaContent(messages []*schema.Message) bool {
    for _, msg := range messages {
        // 检查 MultiContent (向后兼容)
        if len(msg.MultiContent) > 0 {
            return true
        }
        // 检查 Content 中是否包含 markdown 图片语法: ![...](...)
        if strings.Contains(msg.Content, "![") && strings.Contains(msg.Content, "](") {
            return true
        }
    }
    return false
}
```

**说明**: 这是一个关键修复！如果不修改这个方法，OCR 和 Vision 工具将永远不会被启用，因为方案 B 下图片信息在 `Content` 而不是 `MultiContent` 中。

#### 5. System Prompt 优化 (prompt.go:611-699) ✅ 已完成
**修改内容**: 在 APPEND_PROMPT_CN 和 APPEND_PROMPT_EN 中添加图片处理规则

新增内容包括:
1. **图片处理规则总述**: 说明图片格式为 `![图片N](url)`
2. **OCR 工具使用场景**: 明确列出提取、识别、读取文字的场景
3. **Vision 工具使用场景**: 明确列出理解场景、物体、活动的场景
4. **工具调用要点**:
   - 如何从 markdown 语法中提取 URL
   - 如何将 URL 传递给工具
   - 支持批量处理多张图片

**优化效果**:
- 让 LLM 明确知道何时使用 OCR，何时使用 Vision
- 指导 LLM 如何从消息中提取图片 URL
- 减少工具选择错误的可能性

#### 6. ConvertMessageContextToEinoMessages (ai.go:951-973) ⚠️ 关键修复
**修改内容**: 将 MultiContent 中的图片转换为 markdown 格式

```go
// 修改前: 直接复制 MultiContent
if len(msgCtx.MultiContent) > 0 {
    einoMsg.MultiContent = make([]schema.ChatMessagePart, len(msgCtx.MultiContent))
    for i, part := range msgCtx.MultiContent {
        einoMsg.MultiContent[i] = schema.ChatMessagePart{
            Type: schema.ChatMessagePartType(part.Type),
            Text: part.Text,
        }
        if part.ImageURL != nil {
            einoMsg.MultiContent[i].ImageURL = &schema.ChatMessageImageURL{
                URL:    part.ImageURL.URL,
                Detail: schema.ImageURLDetail(part.ImageURL.Detail),
            }
        }
    }
}

// 修改后: 将图片转换为 markdown 格式附加到 Content
if len(msgCtx.MultiContent) > 0 {
    imageCount := 0
    for _, part := range msgCtx.MultiContent {
        // 处理图片：转换为 markdown 格式
        if part.Type == goopenai.ChatMessagePartTypeImageURL && part.ImageURL != nil {
            if einoMsg.Content != "" {
                einoMsg.Content += "\n\n"
            }
            imageCount++
            einoMsg.Content += fmt.Sprintf("![图片%d](%s)\n", imageCount, part.ImageURL.URL)
        } else if part.Type == goopenai.ChatMessagePartTypeText && part.Text != "" {
            // 处理文本：附加到 Content
            if einoMsg.Content != "" {
                einoMsg.Content += "\n"
            }
            einoMsg.Content += part.Text
        }
    }
    // 不再设置 MultiContent
}
```

**说明**: 这个修复确保了从 `MessageContext` 转换到 `schema.Message` 时，图片信息被正确转换为 markdown 格式。这对于旧的会话上下文生成逻辑（`GenChatSessionContextAndSummaryIfExceedsTokenLimit`）非常重要。

### Phase 2: 测试 (已完成基础测试)
- [x] 单元测试 - 基础测试通过
- [x] 编译测试 - 编译成功，无错误
- [ ] OCR 场景集成测试 - 待进行
- [ ] Vision 场景集成测试 - 待进行
- [ ] 混合场景集成测试 - 待进行

### Phase 3: 优化和调整 ✅ (已完成)
- [x] 优化 System Prompt - 添加图片处理规则说明
- [ ] 根据测试结果继续优化工具描述
- [ ] 性能优化

### Phase 4: 部署和监控 (待进行)
- [ ] 部署到测试环境
- [ ] 收集用户反馈
- [ ] 监控 Token 使用和性能指标

## 8. 关键决策点 (已确认)

### 决策 1: 图片 URL 传递方式 ✅
- [ ] **方案 A**: 保留 MultiContent,让 LLM 从中提取 URL
- [x] **方案 B**: 在消息文本中显式包含图片 URL (已采用)

**选择理由**: 非 Vision Model 的 LLM 没有能力接收 MultiContent 中的图片 URL，必须在消息文本中显式提供。使用 markdown 图片语法 `![alt](url)` 既保持了可读性，也方便 LLM 提取 URL。

### 决策 2: 工具启用条件 ✅
- [x] **方案 A**: 只有消息包含多媒体时才启用工具 (已采用)
- [ ] **方案 B**: 始终启用工具,让 LLM 自行判断是否需要

**选择理由**: 避免不必要的工具加载，减少 LLM 的 token 消耗和推理复杂度。

### 决策 3: 回退策略 ✅
- [x] **方案 A**: 完全替换旧逻辑 (已采用)
- [ ] **方案 B**: 添加功能开关,支持新旧模式切换

**选择理由**: 直接替换，简化代码逻辑。如果后续发现问题可以通过 git revert 快速回退。

## 9. 相关文件清单

### 需要修改的文件:
1. `app/logic/v1/auto_assistant.go` - 核心逻辑修改
2. `app/core/prompt_manager.go` - System Prompt 完善 (如果存在)
3. 配置文件 (如果需要添加功能开关)

### 需要测试的文件:
1. `pkg/ai/tools/ocr/ocr_tool.go`
2. `pkg/ai/tools/vision/vision_tool.go`
3. `app/logic/v1/auto_assistant_test.go`

### 相关依赖:
1. `pkg/types/chat_message.go` - 消息格式定义
2. `app/core/srv/ai.go` - AI 服务配置

## 10. 后续优化方向

1. **智能工具组合**: 让 LLM 能够先用 OCR 提取文字,再用 Vision 理解场景
2. **工具结果缓存**: 对相同图片的工具调用结果进行缓存
3. **成本优化**: 根据用户问题的复杂度选择合适的模型
4. **用户反馈机制**: 让用户能够纠正工具选择,用于模型优化
