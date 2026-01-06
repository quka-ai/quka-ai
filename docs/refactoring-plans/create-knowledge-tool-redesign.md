# CreateKnowledgeTool 重构计划

## 问题描述

当前的 `CreateKnowledgeTool` 设计存在以下问题：

1. **依赖模型生成内容**：要求 AI 模型直接在 function call 参数中提供 `content`，这导致：

   - 模型需要在调用工具时就完成内容的总结和格式化
   - 无法独立控制总结的 prompt 和质量
   - 总结的格式和风格完全依赖于聊天模型的理解

2. **缺乏后端控制**：
   - 无法在后端统一设计专门的总结 prompt
   - 无法确保生成的 knowledge 内容符合统一的格式标准
   - 难以优化和迭代总结逻辑

## 改造目标

重新设计 `CreateKnowledgeTool`，使其：

1. **简化参数**：移除 `content` 参数，只保留 `resource` 和 `title` 等元数据参数
2. **后端自主处理**：工具内部自动获取当前 session 的聊天历史
3. **独立总结 prompt**：设计专门的 prompt 来生成高质量的 knowledge 总结
4. **统一格式**：确保生成的 knowledge 内容格式统一，便于检索和展示

## 详细实施方案

### 1. 修改工具参数定义

**当前参数**：

```go
type CreateKnowledgeParams struct {
    Content  string `json:"content"`  // 需要模型提供
    Resource string `json:"resource"`
    Title    string `json:"title"`
}
```

**修改后参数**：

```go
type CreateKnowledgeParams struct {
    Resource string `json:"resource"`
    Title    string `json:"title"`
    // 移除 Content 参数
}
```

**工具描述更新**：

```go
func (t *CreateKnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    params := map[string]*schema.ParameterInfo{
        "resource": {
            Type:     schema.String,
            Desc:     "资源分类ID,不指定则使用默认分类 'knowledge'。建议先使用 ListUserResources 查看可用选项",
            Required: false,
        },
        "title": {
            Type:     schema.String,
            Desc:     "知识标题（可选）。如果不提供，系统会根据聊天内容自动生成",
            Required: false,
        },
    }

    return &schema.ToolInfo{
        Name:        FUNCTION_NAME_CREATE_KNOWLEDGE,
        Desc:        "基于当前对话历史创建知识(记忆)条目。系统会自动总结当前会话的关键信息，生成结构化的知识内容。Resource 是知识的分类标识,如果不指定,将保存到默认分类(knowledge)。",
        ParamsOneOf: paramsOneOf,
    }, nil
}
```

### 2. 实现聊天历史获取（复用现有逻辑）

在 `InvokableRun` 方法中添加聊天历史获取逻辑，**参考现有的 `GenChatSessionContextAndSummaryIfExceedsTokenLimit` 方法**：

```go
func (t *CreateKnowledgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // 1. 解析参数
    var params CreateKnowledgeParams
    if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
        return "", fmt.Errorf("Invalid parameters: %w", err)
    }

    // 2. 并发控制 - 使用分布式信号量限制同时生成总结的用户数
    semaphore := t.core.Semaphore().KnowledgeSummary()
    if !semaphore.TryAcquire() {
        return "", fmt.Errorf("System is busy generating knowledge summaries. Please try again in a moment.")
    }
    defer semaphore.Release()

    // 3. 获取当前会话的聊天历史（从最近的总结往后获取）
    chatHistory, err := t.getChatHistoryFromLastSummary(ctx)
    if err != nil {
        return "", fmt.Errorf("failed to get chat history: %w", err)
    }

    if len(chatHistory) == 0 {
        return "", fmt.Errorf("No chat history available to create knowledge")
    }

    // 4. 使用 AI 生成知识总结
    knowledgeContent, generatedTitle, err := t.generateKnowledgeSummary(ctx, chatHistory, params.Title)
    if err != nil {
        return "", fmt.Errorf("failed to generate knowledge summary: %w", err)
    }

    // 5. 如果用户没有提供标题，使用生成的标题
    if params.Title == "" {
        params.Title = generatedTitle
    }

    // 6. 匹配 resource（保持现有逻辑）
    resource := params.Resource
    if resource == "" || strings.ToLower(resource) == "knowledge" {
        resource = types.DEFAULT_RESOURCE
    } else {
        matchedResourceID, err := t.matchResourceByDescription(ctx, resource)
        if err != nil {
            return fmt.Sprintf("Failed to match resource: %s", err.Error()), nil
        }
        if matchedResourceID == "" {
            return fmt.Sprintf("Could not find a matching resource for '%s'. Use ListUserResources to see available resources.", resource), nil
        }
        resource = matchedResourceID
    }

    // 7. 创建 knowledge
    knowledgeID, err := t.knowledgeFuncs.InsertContentAsyncWithSource(
        t.spaceID,
        resource,
        types.KNOWLEDGE_KIND_TEXT,
        types.KnowledgeContent(knowledgeContent),
        types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
        types.KNOWLEDGE_SOURCE_CHAT,
        t.sessionID,
    )
    if err != nil {
        return "", fmt.Errorf("failed to create knowledge: %w", err)
    }

    // 8. 记录日志
    slog.Info("CreateKnowledgeTool: knowledge created from chat history",
        slog.String("user_id", t.userID),
        slog.String("space_id", t.spaceID),
        slog.String("knowledge_id", knowledgeID),
        slog.String("resource", resource),
        slog.String("title", params.Title),
        slog.Int("content_length", len(knowledgeContent)),
        slog.Int("history_message_count", len(chatHistory)),
    )

    // 9. 返回结果
    return fmt.Sprintf("Knowledge created successfully from conversation history!\nID: %s\nTitle: %s\nResource: %s\nThe knowledge is being processed (summarization and embedding) in background.",
        knowledgeID, params.Title, resource), nil
}
```

### 3. 实现聊天历史获取方法（参考现有逻辑）

**关键：复用现有的 chat summary 逻辑**，从最近的总结开始获取历史记录：

```go
// getChatHistoryFromLastSummary 获取从最近一次总结到当前的聊天历史
// 参考 ai.go 中的 GenChatSessionContextAndSummaryIfExceedsTokenLimit 方法
func (t *CreateKnowledgeTool) getChatHistoryFromLastSummary(ctx context.Context) ([]*types.MessageContext, error) {
    // 1. 获取最近的总结（如果存在）
    summary, err := t.core.Store().ChatSummaryStore().GetChatSessionLatestSummary(ctx, t.sessionID)
    if err != nil && err != sql.ErrNoRows {
        return nil, fmt.Errorf("failed to get latest summary: %w", err)
    }

    var summarySequence int64 = 0
    if summary != nil {
        summarySequence = summary.Sequence
    }

    // 2. 获取比 summary sequence 更大的聊天内容
    msgList, err := t.core.Store().ChatMessageStore().ListSessionMessage(
        ctx,
        t.spaceID,
        t.sessionID,
        summarySequence, // 从总结之后开始获取
        types.NO_PAGINATION,
        types.NO_PAGINATION,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to list session messages: %w", err)
    }

    // 3. 按 sequence 排序
    sort.Slice(msgList, func(i, j int) bool {
        return msgList[i].Sequence < msgList[j].Sequence
    })

    // 4. 转换为 MessageContext 并解密
    var messageContexts []*types.MessageContext
    for _, msg := range msgList {
        // 解密消息（如果加密）
        if msg.IsEncrypt == types.MESSAGE_IS_ENCRYPT {
            decrypted, err := t.core.DecryptData([]byte(msg.Message))
            if err != nil {
                slog.Error("Failed to decrypt message for knowledge creation",
                    slog.String("message_id", msg.ID),
                    slog.String("error", err.Error()))
                continue
            }
            msg.Message = string(decrypted)
        }

        // 只包含完成的消息
        if msg.Complete != types.MESSAGE_PROGRESS_COMPLETE {
            continue
        }

        // 跳过错误消息
        if isErrorMessage(msg.Message) {
            continue
        }

        messageContexts = append(messageContexts, &types.MessageContext{
            Role:    msg.Role,
            Content: msg.Message,
        })
    }

    return messageContexts, nil
}

// isErrorMessage 检查消息是否为错误消息（参考 ai.go）
func isErrorMessage(msg string) bool {
    msg = strings.TrimSpace(msg)
    if strings.HasPrefix(msg, "Sorry，") || strings.HasPrefix(msg, "抱歉，") || msg == "" {
        return true
    }
    return false
}
```

### 4. 设计知识总结 Prompt

````go
// generateKnowledgeSummary 使用 AI 从聊天历史生成知识总结
func (t *CreateKnowledgeTool) generateKnowledgeSummary(ctx context.Context, chatHistory []*types.MessageContext, userProvidedTitle string) (content string, title string, err error) {
    // 1. 验证输入
    if len(chatHistory) == 0 {
        return "", "", fmt.Errorf("chat history is empty")
    }

    // 2. 构建聊天历史的文本表示
    var historyBuilder strings.Builder
    for _, msg := range chatHistory {
        role := "User"
        if msg.Role == types.USER_ROLE_ASSISTANT {
            role = "Assistant"
        } else if msg.Role == types.USER_ROLE_SYSTEM {
            role = "System"
        } else if msg.Role == types.USER_ROLE_TOOL {
            role = "Tool"
        }

        historyBuilder.WriteString(fmt.Sprintf("%s: %s\n\n", role, msg.Content))
    }

    // 2. 构建系统 prompt
    systemPrompt := `你是一个专业的知识总结助手。你的任务是从对话历史中提取关键信息，生成结构化的知识条目。

要求：
1. 使用标准的 Markdown 格式
2. 提取对话中的核心概念、重要信息和结论
3. 组织成清晰的结构（使用标题、列表、代码块等）
4. 保留重要的技术细节和上下文
5. 去除无关的对话内容（如问候语、确认等）
6. 如果对话包含代码，请使用代码块格式化
7. 如果对话包含步骤，使用有序或无序列表
8. 总结应该独立可读，不依赖原始对话

输出格式：
- 如果用户已提供标题，直接输出内容（JSON 格式：{"content": "...", "title": ""}）
- 如果用户未提供标题，同时生成标题和内容（JSON 格式：{"content": "...", "title": "..."}）
- 标题应简洁明了，反映知识的核心主题（不超过50字符）
- 内容必须是有效的 Markdown 格式`

    // 3. 构建用户 prompt
    titleInstruction := "请为这段对话生成一个简洁的标题和结构化的总结。"
    if userProvidedTitle != "" {
        titleInstruction = fmt.Sprintf("用户已提供标题：'%s'。请生成结构化的总结内容。", userProvidedTitle)
    }

    userPrompt := fmt.Sprintf(`%s

对话历史：
%s

请以 JSON 格式返回结果：
{"content": "markdown格式的总结内容", "title": "标题（如果用户已提供则留空）"}`,
        titleInstruction, historyBuilder.String())

    // 4. 调用 AI 模型
    chatModel := t.core.Srv().AI().GetChatAI(false)
    messages := []*schema.Message{
        {
            Role:    schema.System,
            Content: systemPrompt,
        },
        {
            Role:    schema.User,
            Content: userPrompt,
        },
    }

    resp, err := chatModel.Generate(ctx, messages)
    if err != nil {
        return "", "", fmt.Errorf("AI service error: %w", err)
    }

    // 5. 记录 token 使用量
    go safe.Run(func() {
        t.recordTokenUsage(ctx, chatModel, resp, "knowledge_summarization")
    })

    // 6. 解析响应
    var result struct {
        Content string `json:"content"`
        Title   string `json:"title"`
    }

    // 尝试清理可能的 markdown 代码块标记
    cleanedContent := strings.TrimSpace(resp.Content)
    cleanedContent = strings.TrimPrefix(cleanedContent, "```json")
    cleanedContent = strings.TrimPrefix(cleanedContent, "```")
    cleanedContent = strings.TrimSuffix(cleanedContent, "```")
    cleanedContent = strings.TrimSpace(cleanedContent)

    if err := json.Unmarshal([]byte(cleanedContent), &result); err != nil {
        return "", "", fmt.Errorf("failed to parse AI response: %w", err)
    }

    // 7. 验证内容
    if result.Content == "" {
        return "", "", fmt.Errorf("AI generated empty content")
    }

    // 8. 如果用户提供了标题，使用用户的标题；否则使用生成的标题
    finalTitle := userProvidedTitle
    if finalTitle == "" {
        finalTitle = result.Title
    }
    if finalTitle == "" {
        finalTitle = "Untitled Knowledge"
    }

    slog.Info("Knowledge summary generated",
        slog.String("user_id", t.userID),
        slog.String("session_id", t.sessionID),
        slog.String("generated_title", result.Title),
        slog.String("final_title", finalTitle),
        slog.Int("content_length", len(result.Content)),
    )

    return result.Content, finalTitle, nil
}
````

### 5. 实现分布式信号量（并发控制）

为了防止过多用户同时生成总结导致 model limit，需要实现基于 Redis 的分布式信号量：

#### 5.1 在 `pkg/types/protocol/redis.go` 中添加 Key 生成函数

```go
// GenKnowledgeSummaryGlobalSemaphoreKey 全局知识总结信号量 key
func GenKnowledgeSummaryGlobalSemaphoreKey() string {
    return fmt.Sprintf("%sknowledge_summary_semaphore", REDIS_CACHE_KEY_PREFIX)
}
```

#### 5.2 实现分布式信号量

在 `app/core` 或 `pkg/utils` 中创建 `semaphore.go`：

```go
package core

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// DistributedSemaphore 分布式信号量，基于 Redis 实现
type DistributedSemaphore struct {
    redis      redis.UniversalClient
    key        string
    maxPermits int
    timeout    time.Duration
}

// NewDistributedSemaphore 创建分布式信号量
func NewDistributedSemaphore(redis redis.UniversalClient, key string, maxPermits int, timeout time.Duration) *DistributedSemaphore {
    return &DistributedSemaphore{
        redis:      redis,
        key:        key,
        maxPermits: maxPermits,
        timeout:    timeout,
    }
}

// TryAcquire 尝试获取信号量许可
func (s *DistributedSemaphore) TryAcquire() bool {
    ctx := context.Background()

    // 使用 Lua 脚本保证原子性
    script := `
        local key = KEYS[1]
        local max_permits = tonumber(ARGV[1])
        local timeout = tonumber(ARGV[2])

        local current = tonumber(redis.call('GET', key) or '0')

        if current < max_permits then
            redis.call('INCR', key)
            redis.call('EXPIRE', key, timeout)
            return 1
        else
            return 0
        end
    `

    result, err := s.redis.Eval(ctx, script, []string{s.key}, s.maxPermits, int(s.timeout.Seconds())).Int()
    if err != nil {
        return false
    }

    return result == 1
}

// Release 释放信号量许可
func (s *DistributedSemaphore) Release() {
    ctx := context.Background()

    // 使用 Lua 脚本保证原子性，避免减到负数
    script := `
        local key = KEYS[1]
        local current = tonumber(redis.call('GET', key) or '0')

        if current > 0 then
            redis.call('DECR', key)
            return 1
        else
            return 0
        end
    `

    s.redis.Eval(ctx, script, []string{s.key})
}

// GetCurrent 获取当前已使用的许可数
func (s *DistributedSemaphore) GetCurrent() int {
    ctx := context.Background()
    result, err := s.redis.Get(ctx, s.key).Int()
    if err != nil {
        return 0
    }
    return result
}
```

#### 5.3 创建统一的信号量管理器

在 `app/core/semaphore.go` 中添加信号量管理器：

```go
// SemaphoreManager 信号量管理器，统一管理所有分布式信号量
type SemaphoreManager struct {
    core              *Core
    knowledgeSummary  *DistributedSemaphore
    knowledgeSummaryOnce sync.Once
}

// NewSemaphoreManager 创建信号量管理器
func NewSemaphoreManager(core *Core) *SemaphoreManager {
    return &SemaphoreManager{
        core: core,
    }
}

// KnowledgeSummary 获取知识总结信号量（懒加载）
// 默认限制：同时最多 10 个用户可以生成知识总结
func (m *SemaphoreManager) KnowledgeSummary() *DistributedSemaphore {
    m.knowledgeSummaryOnce.Do(func() {
        maxConcurrency := 10 // 默认值
        if m.core.cfg.Knowledge.SummaryMaxConcurrency > 0 {
            maxConcurrency = m.core.cfg.Knowledge.SummaryMaxConcurrency
        }

        m.knowledgeSummary = NewDistributedSemaphore(
            m.core.Redis(),
            protocol.GenKnowledgeSummaryGlobalSemaphoreKey(),
            maxConcurrency,
            time.Minute*5, // 5分钟超时
        )
    })
    return m.knowledgeSummary
}

// 未来可以添加更多信号量，例如：
// func (m *SemaphoreManager) FileUpload() *DistributedSemaphore { ... }
// func (m *SemaphoreManager) AIGeneration() *DistributedSemaphore { ... }
```

在 `app/core/core.go` 中添加：

```go
type Core struct {
    // ... 其他字段
    semaphoreManager *SemaphoreManager
}

// Semaphore 获取信号量管理器
func (c *Core) Semaphore() *SemaphoreManager {
    if c.semaphoreManager == nil {
        c.semaphoreManager = NewSemaphoreManager(c)
    }
    return c.semaphoreManager
}
```

#### 5.4 配置文件支持

在 `app/core/config.go` 中添加配置：

```go
type KnowledgeConfig struct {
    SummaryMaxConcurrency int `toml:"summary_max_concurrency"` // 知识总结最大并发数
}

type Config struct {
    // ... 其他配置
    Knowledge KnowledgeConfig `toml:"knowledge"`
}
```

在配置文件 `config.toml` 中添加：

```toml
[knowledge]
# 知识总结最大并发数，默认 10
summary_max_concurrency = 10
```

### 6. 更新依赖注入

确保 `CreateKnowledgeTool` 结构体已经包含了访问必要资源的能力：

```go
type CreateKnowledgeTool struct {
    core           *core.Core  // 通过 core 可以访问 Store、Redis、Semaphore
    spaceID        string
    sessionID      string      // 已有，用于获取聊天历史
    userID         string
    knowledgeFuncs KnowledgeLogicFunctions
    resourceFuncs  ResourceLogicFunctions
}
```

由于 `core *core.Core` 已经存在，可以通过：

- `t.core.Store().ChatMessageStore()` - 访问聊天消息存储
- `t.core.Store().ChatSummaryStore()` - 访问聊天总结存储
- `t.core.Semaphore().KnowledgeSummary()` - 访问知识总结分布式信号量

无需额外修改结构体。

## 关键考虑点

### 1. 聊天历史范围（已解决）

✅ **复用现有的 summary 逻辑**：从最近一次总结开始获取历史记录到当前

- 这与现有的 chat 系统一致
- 避免获取过长的历史记录
- 自动利用已有的总结机制

### 2. 并发控制（已解决）

✅ **使用分布式信号量**：

- 基于 Redis 实现
- 默认允许 10 个并发（可配置）
- 超时时间 5 分钟
- 防止过多用户同时生成总结导致 model limit

### 3. 模型选择（已确定）

✅ **使用 `srv.AI().GetChatAI(false)` 获取模型**：

- 与现有的聊天模型保持一致
- 不需要单独配置模型

### 4. 错误处理（已确定）

✅ **Tool 调用直接返回错误**：

- 聊天历史为空 → 返回错误
- AI 生成失败 → 返回错误
- 信号量获取失败 → 返回错误（系统繁忙）
- 不需要降级方案

### 5. 消息加密和过滤（已处理）

✅ **参考现有逻辑**：

- 自动解密加密的消息
- 跳过未完成的消息
- 跳过错误消息
- 处理 tool 消息

### 6. Token 使用记录

✅ **复用现有的 `recordTokenUsage` 方法**：

- 记录总结生成的 token 使用
- `SubType`: `knowledge_summarization`
- 便于后续分析成本

## 实施步骤

### 第一步：实现分布式信号量基础设施

1. 在 `pkg/types/protocol/redis.go` 中添加 `GenKnowledgeSummaryGlobalSemaphoreKey()`
2. 在 `app/core/semaphore.go` 中实现：
   - `DistributedSemaphore` 结构体和方法
   - `SemaphoreManager` 信号量管理器
3. 在 `app/core/config.go` 中添加 `KnowledgeConfig` 配置
4. 在 `app/core/core.go` 中添加 `Semaphore()` 方法返回信号量管理器

### 第二步：修改 CreateKnowledgeTool

1. 修改 `CreateKnowledgeParams` 结构体，移除 `Content` 字段
2. 更新 `Info()` 方法的参数描述和工具描述
3. 重写 `InvokableRun()` 方法：
   - 添加信号量并发控制
   - 调用 `getChatHistoryFromLastSummary()` 获取历史
   - 调用 `generateKnowledgeSummary()` 生成总结
   - 创建 knowledge

### 第三步：实现辅助方法

1. 实现 `getChatHistoryFromLastSummary()` - 获取聊天历史
2. 实现 `generateKnowledgeSummary()` - 生成总结
3. 实现 `isErrorMessage()` - 检查错误消息

### 第四步：测试

1. 单元测试：信号量的获取和释放
2. 集成测试：完整的知识创建流程
3. 性能测试：并发场景下的表现
4. 边界测试：空历史、长历史、加密消息等

### 第五步：更新相关代码

1. 检查 MCP tools 中是否有类似实现需要同步更新
2. 更新前端调用代码（移除 content 参数）
3. 更新 API 文档

## 相关文件

### 需要新建的文件：

- `app/core/semaphore.go` - 分布式信号量实现（包含 `DistributedSemaphore` 和 `SemaphoreManager`）

### 需要修改的文件：

- [pkg/ai/agents/knowledge/function.go](../../pkg/ai/agents/knowledge/function.go) - 主要修改文件（CreateKnowledgeTool）
- [pkg/types/protocol/redis.go](../../pkg/types/protocol/redis.go) - 添加 Redis key 生成函数
- [app/core/config.go](../../app/core/config.go) - 添加 Knowledge 配置
- [app/core/core.go](../../app/core/core.go) - 添加信号量访问方法

### 需要参考的文件：

- [app/logic/v1/ai.go](../../app/logic/v1/ai.go) - 参考 `GenChatSessionContextAndSummaryIfExceedsTokenLimit` 和 `GenChatSessionContextSummary`
- [app/logic/v1/chat_history.go](../../app/logic/v1/chat_history.go) - 聊天历史逻辑
- [app/store/sqlstore/chat_message.go](../../app/store/sqlstore/chat_message.go) - 消息存储接口
- [app/store/sqlstore/chat_summary.go](../../app/store/sqlstore/chat_summary.go) - 总结存储接口

### 可能需要同步更新的文件：

- [pkg/mcp/tools/knowledge.go](../../pkg/mcp/tools/knowledge.go) - MCP 工具中的 knowledge 创建

## 优势分析

### 相比原方案的改进：

1. **后端可控**：

   - ✅ 完全控制总结的质量和格式
   - ✅ 可以独立优化总结 prompt
   - ✅ 统一的知识内容标准

2. **用户体验**：

   - ✅ 简化了工具调用，只需指定 resource 和 title
   - ✅ 自动从聊天历史生成总结
   - ✅ 支持自动生成标题

3. **系统稳定性**：

   - ✅ 分布式信号量防止 model limit
   - ✅ 复用现有的 summary 逻辑，减少重复代码
   - ✅ 自动处理加密消息

4. **可维护性**：
   - ✅ 代码结构清晰，职责分明
   - ✅ 易于测试和调试
   - ✅ 便于后续迭代和优化

## 风险和注意事项

1. **兼容性**：

   - ⚠️ 前端需要同步更新，移除 `content` 参数
   - ⚠️ 需要通知使用 API 的第三方开发者

2. **性能**：

   - ⚠️ 每次调用都需要查询聊天历史和总结
   - ⚠️ AI 总结生成需要额外的 token 消耗
   - ✅ 通过信号量控制并发，避免系统过载

3. **数据质量**：
   - ⚠️ 如果聊天历史很短，生成的总结质量可能不佳
   - ⚠️ 依赖 AI 模型的总结能力
   - ✅ 可以通过优化 prompt 提高质量

## 总结

本次重构的核心目标是让 `CreateKnowledgeTool` 更加智能和自动化：

1. **移除 content 参数**：不再要求模型直接提供内容
2. **自动获取历史**：从最近的总结开始获取聊天历史
3. **后端生成总结**：使用专门设计的 prompt 生成高质量总结
4. **并发控制**：使用分布式信号量防止系统过载
5. **复用现有逻辑**：最大程度复用 chat summary 的现有机制

这样的设计既保证了知识质量，又提升了用户体验，同时保障了系统稳定性。
