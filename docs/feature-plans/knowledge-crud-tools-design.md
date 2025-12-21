# Knowledge CRUD Tools 设计方案

## 背景说明

当前项目使用 [Eino 框架](https://github.com/cloudwego/eino) 实现 AI Agent 的 tool calling 机制。现有的 knowledge 相关 tool 只有查询功能(通过 RAG tool),需要补充创建(Create)和更新(Update)功能,让大模型能够帮助用户管理 knowledge。

## 核心挑战：Resource 粒度的交互设计

### 问题分析

Knowledge 的数据模型有两层分类:
1. **Space 级别**: 在 tool 实例化时通过 `spaceID` 参数确定(来自聊天上下文)
2. **Resource 级别**: knowledge 需要归属到某个 resource 下,但这个信息需要在运行时确定

关键难点在于:**如何让大模型智能地选择合适的 resource,同时保持交互的简洁性?**

### 设计方案

#### 方案一:智能默认 + 辅助查询(推荐)

**核心思想**:
- 大部分场景使用默认 resource("knowledge"),用户无需关心分类
- 提供 `ListUserResources` 辅助 tool,让大模型能够发现可用的 resources
- 在 create/update tool 的描述中引导大模型先查询可用 resources

**优点**:
- 简单场景下用户体验流畅(直接创建,无需选择)
- 高级场景下大模型能够智能选择合适的 resource
- 灵活性高,可扩展

**缺点**:
- 需要两次 tool 调用(先查询 resources,再创建/更新)
- 依赖大模型的理解能力

#### 方案二:交互式确认

**核心思想**:
- 创建 knowledge 时,如果不指定 resource,返回可用 resources 列表
- 要求用户或大模型二次确认选择

**优点**:
- 用户明确知道 knowledge 的分类
- 避免误分类

**缺点**:
- 每次创建都需要确认,体验不流畅
- 实现复杂,需要维护对话状态

#### 方案三:语义自动匹配

**核心思想**:
- 根据 knowledge 的内容,自动推断最合适的 resource
- 使用 LLM 分析内容语义和 resource 描述的匹配度

**优点**:
- 用户体验最佳,完全自动化
- 智能化程度高

**缺点**:
- 实现复杂,需要额外的 LLM 调用
- 准确性难以保证
- 性能开销大

### 最终选择:**方案一(智能默认 + 辅助查询)**

**理由**:
1. **平衡性好**: 兼顾简单场景和复杂场景
2. **实现清晰**: 利用现有的 `ResourceLogic.ListUserResources` 方法
3. **可控性强**: 大模型可以根据上下文决定是否查询 resources
4. **扩展性好**: 未来可以基于此增强(如添加语义匹配作为建议)

## Tool 设计

### 1. CreateKnowledgeTool - 创建知识

**功能**: 在用户的知识库中创建新的 knowledge 条目

**参数设计**:
```go
type CreateKnowledgeParams struct {
    Content     string   `json:"content"`     // 必填: 知识内容(markdown格式)
    Resource    string   `json:"resource"`    // 可选: resource ID,不指定则使用"knowledge"
    Title       string   `json:"title"`       // 可选: 标题
    Tags        []string `json:"tags"`        // 可选: 标签
    ContentType string   `json:"contentType"` // 可选: 内容类型(markdown/blocks)
}
```

**Tool 描述**(给大模型看的):
```
创建新的知识条目。知识内容必须使用标准的 Markdown 格式。

Resource 说明:
- resource 是知识的分类标识,用于组织和管理知识
- 如果不指定 resource,知识将保存到默认分类 "knowledge" 下
- 建议先使用 ListUserResources 工具查看可用的 resource,选择合适的分类
- 常见的 resource 类型:项目文档、会议记录、学习笔记等

使用建议:
- 快速记录:直接创建,无需指定 resource
- 结构化管理:先查询可用 resources,选择合适的分类
```

**实现要点**:
```go
type CreateKnowledgeTool struct {
    core    *core.Core
    spaceID string
    userID  string
}

func (t *CreateKnowledgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // 1. 解析参数
    var params CreateKnowledgeParams
    json.Unmarshal([]byte(argumentsInJSON), &params)

    // 2. 参数默认值
    resource := params.Resource
    if resource == "" {
        resource = types.DEFAULT_RESOURCE // "knowledge"
    }

    contentType := types.StringToKnowledgeContentType(params.ContentType)
    if contentType == types.KNOWLEDGE_CONTENT_TYPE_UNKNOWN {
        contentType = types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN
    }

    // 3. 验证 resource 存在(如果不是默认值)
    if resource != types.DEFAULT_RESOURCE {
        resourceLogic := v1.NewResourceLogic(ctx, t.core)
        res, err := resourceLogic.GetResource(t.spaceID, resource)
        if err != nil || res == nil {
            return fmt.Sprintf("Resource '%s' not found. Use ListUserResources to see available resources.", resource), nil
        }
    }

    // 4. 创建 knowledge
    logic := v1.NewKnowledgeLogic(ctx, t.core)
    knowledgeID, err := logic.InsertContentAsync(
        t.spaceID,
        resource,
        types.KNOWLEDGE_KIND_TEXT,
        types.KnowledgeContent(params.Content),
        contentType,
    )
    if err != nil {
        return "", fmt.Errorf("failed to create knowledge: %w", err)
    }

    // 5. 返回结果
    return fmt.Sprintf("Knowledge created successfully!\nID: %s\nResource: %s\nThe knowledge is being processed (summarization and embedding) in background.",
        knowledgeID, resource), nil
}
```

**返回示例**:
```
Knowledge created successfully!
ID: abc123def456
Resource: meeting-notes
The knowledge is being processed (summarization and embedding) in background.
```

### 2. UpdateKnowledgeTool - 更新知识

**功能**: 更新已存在的 knowledge 的内容、分类或元数据

**参数设计**:
```go
type UpdateKnowledgeParams struct {
    ID          string   `json:"id"`          // 必填: knowledge ID
    Content     string   `json:"content"`     // 可选: 新内容(markdown格式)
    Resource    string   `json:"resource"`    // 可选: 移动到新的 resource
    Title       string   `json:"title"`       // 可选: 新标题
    Tags        []string `json:"tags"`        // 可选: 新标签
    ContentType string   `json:"contentType"` // 可选: 内容类型
}
```

**Tool 描述**:
```
更新已存在的知识条目。只需提供要更新的字段,未提供的字段保持不变。

参数说明:
- id: 要更新的 knowledge ID(必填)
- content: 如果提供,知识将被重新处理(summarization + embedding)
- resource: 如果提供,知识将被移动到指定的 resource 分类
- title/tags: 更新元数据

注意事项:
- 更新 resource 时,建议先使用 ListUserResources 确认目标 resource 存在
- 更新 content 会触发异步处理,可能需要几秒钟
- 只能更新属于当前用户的 knowledge
```

**实现要点**:
```go
type UpdateKnowledgeTool struct {
    core    *core.Core
    spaceID string
    userID  string
}

func (t *UpdateKnowledgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // 1. 解析参数
    var params UpdateKnowledgeParams
    json.Unmarshal([]byte(argumentsInJSON), &params)

    if params.ID == "" {
        return "Error: knowledge ID is required", nil
    }

    // 2. 验证 knowledge 存在且属于当前用户
    logic := v1.NewKnowledgeLogic(ctx, t.core)
    existing, err := logic.GetKnowledge(t.spaceID, params.ID)
    if err != nil {
        return fmt.Sprintf("Knowledge not found: %s", params.ID), nil
    }

    if existing.UserID != t.userID {
        return "Error: You don't have permission to update this knowledge", nil
    }

    // 3. 构建更新参数
    updateArgs := types.UpdateKnowledgeArgs{}
    updatedFields := []string{}

    if params.Title != "" {
        updateArgs.Title = params.Title
        updatedFields = append(updatedFields, "title")
    }

    if params.Content != "" {
        contentType := types.StringToKnowledgeContentType(params.ContentType)
        if contentType == types.KNOWLEDGE_CONTENT_TYPE_UNKNOWN {
            contentType = existing.ContentType // 保持原有类型
        }
        updateArgs.Content = types.KnowledgeContent(params.Content)
        updateArgs.ContentType = contentType
        updatedFields = append(updatedFields, "content")
    }

    if len(params.Tags) > 0 {
        updateArgs.Tags = params.Tags
        updatedFields = append(updatedFields, "tags")
    }

    if params.Resource != "" && params.Resource != existing.Resource {
        // 验证目标 resource 存在
        if params.Resource != types.DEFAULT_RESOURCE {
            resourceLogic := v1.NewResourceLogic(ctx, t.core)
            targetResource, err := resourceLogic.GetResource(t.spaceID, params.Resource)
            if err != nil || targetResource == nil {
                return fmt.Sprintf("Target resource '%s' not found. Use ListUserResources to see available resources.",
                    params.Resource), nil
            }
        }
        updateArgs.Resource = params.Resource
        updatedFields = append(updatedFields, "resource")
    }

    if len(updatedFields) == 0 {
        return "No fields to update. Please provide at least one field (content, resource, title, or tags).", nil
    }

    // 4. 执行更新
    err = logic.Update(t.spaceID, params.ID, updateArgs)
    if err != nil {
        return "", fmt.Errorf("failed to update knowledge: %w", err)
    }

    // 5. 返回结果
    status := "updated"
    if params.Content != "" {
        status = "updated and re-processing"
    }

    return fmt.Sprintf("Knowledge %s successfully!\nID: %s\nUpdated fields: %s",
        status, params.ID, strings.Join(updatedFields, ", ")), nil
}
```

**返回示例**:
```
Knowledge updated and re-processing successfully!
ID: abc123def456
Updated fields: content, tags
```

### 3. ListUserResourcesTool - 列出用户资源

**功能**: 列出用户有权访问的所有 resources,帮助大模型了解可用的分类

**参数设计**:
```go
// 无需参数,从tool实例化时的上下文获取 userID
```

**Tool 描述**:
```
列出用户可以使用的所有 resource(知识分类)。

Resource 是用于组织和管理知识的分类标识,每个 resource 可以有:
- 标题:人类可读的名称
- 描述:说明该 resource 的用途
- 周期:知识过期时间(天数),0 表示永不过期

使用场景:
- 在创建或更新 knowledge 前,查看可用的分类选项
- 了解知识库的组织结构
- 选择最合适的 resource 来存储 knowledge
```

**实现要点**:
```go
type ListUserResourcesTool struct {
    core   *core.Core
    userID string
}

func (t *ListUserResourcesTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // 1. 调用 ResourceLogic 获取用户的 resources
    logic := v1.NewResourceLogic(ctx, t.core)
    resources, err := logic.ListUserResources(0, 0) // 不分页,返回所有
    if err != nil {
        return "", fmt.Errorf("failed to list user resources: %w", err)
    }

    if len(resources) == 0 {
        return "You don't have any custom resources yet. Knowledge will be saved to the default resource 'knowledge'.", nil
    }

    // 2. 格式化输出
    sb := strings.Builder{}
    sb.WriteString("Available Resources:\n\n")
    sb.WriteString("| ID | Title | Description | Lifecycle |\n")
    sb.WriteString("| --- | --- | --- | --- |\n")

    for _, r := range resources {
        lifecycle := "Permanent"
        if r.Cycle > 0 {
            lifecycle = fmt.Sprintf("%d days", r.Cycle)
        }

        desc := r.Description
        if desc == "" {
            desc = "-"
        }

        sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
            r.ID, r.Title, desc, lifecycle))
    }

    sb.WriteString("\nUsage:\n")
    sb.WriteString("- Use the 'ID' column value when creating or updating knowledge\n")
    sb.WriteString("- If no resource is specified, knowledge will be saved to 'knowledge'\n")

    return sb.String(), nil
}
```

**返回示例**:
```
Available Resources:

| ID | Title | Description | Lifecycle |
| --- | --- | --- | --- |
| knowledge | 默认知识库 | - | Permanent |
| meeting-notes | 会议记录 | 存储所有团队会议的记录 | 90 days |
| projects | 项目文档 | 项目相关的技术文档和设计 | Permanent |
| learning | 学习笔记 | 个人学习和研究笔记 | Permanent |

Usage:
- Use the 'ID' column value when creating or updating knowledge
- If no resource is specified, knowledge will be saved to 'knowledge'
```

## 工具集成

### 工具注册

```go
// pkg/ai/agents/knowledge/function.go

func GetKnowledgeTools(core *core.Core, spaceID, userID string) []tool.InvokableTool {
    return []tool.InvokableTool{
        NewCreateKnowledgeTool(core, spaceID, userID),
        NewUpdateKnowledgeTool(core, spaceID, userID),
        NewListUserResourcesTool(core, userID),
    }
}
```

### 在 Agent 中使用

```go
// app/logic/v1/auto_assistant.go

type WithKnowledgeTools struct{}

func NewWithKnowledgeTools() *WithKnowledgeTools {
    return &WithKnowledgeTools{}
}

func (o *WithKnowledgeTools) Apply(config *AgentConfig) error {
    knowledgeTools := knowledge.GetKnowledgeTools(
        config.Core,
        config.AgentCtx.SpaceID,
        config.AgentCtx.UserID,
    )

    for _, knowledgeTool := range knowledgeTools {
        notifyingTool := config.ToolWrapper.Wrap(knowledgeTool)
        config.Tools = append(config.Tools, notifyingTool)
    }

    return nil
}
```

## AI 使用场景

### 场景 1: 快速记录想法(使用默认 resource)

**用户**: "帮我记住,明天要完成代码 review"

**AI 工作流**:
```javascript
// 直接创建,使用默认 resource
CreateKnowledge({
  content: "明天要完成代码 review",
  title: "待办事项"
})
// 响应: Knowledge created! ID: abc123, Resource: knowledge
```

**说明**: 简单场景下,AI 无需查询 resources,直接创建即可。

### 场景 2: 结构化管理(先查询 resources)

**用户**: "把这次的项目会议内容记录下来"

**AI 工作流**:
```javascript
// Step 1: 查询可用的 resources
ListUserResources()
// 返回: meeting-notes, projects, learning 等

// Step 2: 选择合适的 resource 创建
CreateKnowledge({
  content: "# 项目进度会议\n\n## 讨论内容\n...",
  resource: "meeting-notes",
  title: "2025-12-18 项目进度会议",
  tags: ["meeting", "project", "2025-q4"]
})
// 响应: Knowledge created! ID: def456, Resource: meeting-notes
```

**说明**: 对于需要分类管理的内容,AI 会先查询 resources,选择最合适的分类。

### 场景 3: 更新现有知识

**用户**: "更新刚才的会议记录,添加待办事项"

**AI 工作流**:
```javascript
// Step 1: 搜索最近的会议记录(使用现有的 RAG tool)
SearchUserKnowledges({
  query: "2025-12-18 项目进度会议"
})
// 找到 knowledge ID: def456

// Step 2: 更新内容
UpdateKnowledge({
  id: "def456",
  content: "# 项目进度会议\n\n## 讨论内容\n...\n\n## 待办事项\n- [ ] 完成API设计\n- [ ] 准备演示文稿"
})
// 响应: Knowledge updated and re-processing! Updated fields: content
```

### 场景 4: 移动知识到不同分类

**用户**: "把那条会议记录移到项目文档里"

**AI 工作流**:
```javascript
// Step 1: 搜索会议记录
SearchUserKnowledges({query: "会议记录"})
// 找到 ID: def456

// Step 2: 移动到 projects resource
UpdateKnowledge({
  id: "def456",
  resource: "projects"
})
// 响应: Knowledge updated! Updated fields: resource
```

### 场景 5: 批量整理知识库

**用户**: "帮我把所有关于 Python 的笔记整理到学习笔记分类下"

**AI 工作流**:
```javascript
// Step 1: 搜索 Python 相关的 knowledge
SearchUserKnowledges({query: "Python"})
// 假设找到 5 条

// Step 2: 批量更新 resource
for (let knowledge of searchResults) {
  UpdateKnowledge({
    id: knowledge.id,
    resource: "learning",
    tags: ["python", "programming"]
  })
}
```

## 技术实现细节

### 1. Tool 结构体定义

```go
// pkg/ai/agents/knowledge/function.go

const (
    FUNCTION_NAME_CREATE_KNOWLEDGE       = "CreateKnowledge"
    FUNCTION_NAME_UPDATE_KNOWLEDGE       = "UpdateKnowledge"
    FUNCTION_NAME_LIST_USER_RESOURCES    = "ListUserResources"
)

// CreateKnowledgeTool 创建知识工具
type CreateKnowledgeTool struct {
    core    *core.Core
    spaceID string
    userID  string
}

func NewCreateKnowledgeTool(core *core.Core, spaceID, userID string) *CreateKnowledgeTool {
    return &CreateKnowledgeTool{
        core:    core,
        spaceID: spaceID,
        userID:  userID,
    }
}

var _ tool.InvokableTool = (*CreateKnowledgeTool)(nil)

// Info 实现 BaseTool 接口
func (t *CreateKnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    params := map[string]*schema.ParameterInfo{
        "content": {
            Type:     schema.String,
            Desc:     "知识内容,必须使用 Markdown 格式",
            Required: true,
        },
        "resource": {
            Type:     schema.String,
            Desc:     "资源分类ID,不指定则使用默认分类 'knowledge'。建议先使用 ListUserResources 查看可用选项",
            Required: false,
        },
        "title": {
            Type:     schema.String,
            Desc:     "知识标题",
            Required: false,
        },
        "tags": {
            Type:     schema.Array,
            Desc:     "标签列表",
            Required: false,
        },
        "contentType": {
            Type:     schema.String,
            Desc:     "内容类型: 'markdown' 或 'blocks',默认为 'markdown'",
            Required: false,
        },
    }

    paramsOneOf := schema.NewParamsOneOfByParams(params)

    return &schema.ToolInfo{
        Name:        FUNCTION_NAME_CREATE_KNOWLEDGE,
        Desc:        "创建新的知识条目。知识内容必须使用标准的 Markdown 格式。Resource 是知识的分类标识,如果不指定,将保存到默认分类。建议先使用 ListUserResources 工具查看可用的分类选项,选择合适的 resource。",
        ParamsOneOf: paramsOneOf,
    }, nil
}

// InvokableRun 实现 InvokableTool 接口
func (t *CreateKnowledgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // 实现见上文
}
```

### 2. 错误处理策略

```go
// 友好的错误返回(不使用 error,而是返回描述性文本)
func (t *CreateKnowledgeTool) InvokableRun(...) (string, error) {
    // 解析参数错误
    var params CreateKnowledgeParams
    if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
        return "Invalid parameters. Please check your input format.", nil
    }

    // 业务逻辑错误
    if params.Content == "" {
        return "Error: content is required", nil
    }

    // Resource 不存在
    if resource := validateResource(...); !resource.Exists {
        return fmt.Sprintf("Resource '%s' not found. Available resources: %s",
            params.Resource, strings.Join(availableResources, ", ")), nil
    }

    // 系统错误才返回 error
    if err := systemCall(); err != nil {
        return "", fmt.Errorf("system error: %w", err)
    }

    return "Success message", nil
}
```

**错误处理原则**:
- **可恢复的错误**: 返回描述性文本,让大模型理解并采取行动
- **系统错误**: 返回 error,中断执行
- **提供建议**: 错误信息中包含解决建议(如列出可用的 resources)

### 3. 参数验证

```go
func validateCreateParams(params CreateKnowledgeParams) (bool, string) {
    // 必填字段检查
    if params.Content == "" {
        return false, "content is required"
    }

    // 内容长度检查
    if len(params.Content) > 100000 {
        return false, "content is too long (max 100KB)"
    }

    // 标签数量检查
    if len(params.Tags) > 20 {
        return false, "too many tags (max 20)"
    }

    return true, ""
}
```

### 4. 日志记录

```go
func (t *CreateKnowledgeTool) InvokableRun(...) (string, error) {
    slog.Info("CreateKnowledgeTool called",
        slog.String("user_id", t.userID),
        slog.String("space_id", t.spaceID),
        slog.String("resource", params.Resource),
        slog.Int("content_length", len(params.Content)),
    )

    // ... 业务逻辑 ...

    slog.Info("Knowledge created",
        slog.String("knowledge_id", knowledgeID),
        slog.String("resource", resource),
    )

    return result, nil
}
```

## 测试计划

### 1. 单元测试

```go
// pkg/ai/agents/knowledge/function_test.go

func TestCreateKnowledgeTool_Info(t *testing.T) {
    tool := NewCreateKnowledgeTool(&core.Core{}, "space123", "user456")

    info, err := tool.Info(context.Background())
    assert.NoError(t, err)
    assert.Equal(t, FUNCTION_NAME_CREATE_KNOWLEDGE, info.Name)
    assert.NotEmpty(t, info.Desc)
    assert.NotNil(t, info.ParamsOneOf)
}

func TestCreateKnowledgeTool_Run_DefaultResource(t *testing.T) {
    // 测试使用默认 resource 创建
}

func TestCreateKnowledgeTool_Run_CustomResource(t *testing.T) {
    // 测试指定 resource 创建
}

func TestCreateKnowledgeTool_Run_InvalidResource(t *testing.T) {
    // 测试无效的 resource
}

func TestUpdateKnowledgeTool_Run_PartialUpdate(t *testing.T) {
    // 测试部分字段更新
}

func TestUpdateKnowledgeTool_Run_MoveResource(t *testing.T) {
    // 测试移动 resource
}

func TestListUserResourcesTool_Run(t *testing.T) {
    // 测试列出 resources
}
```

### 2. 集成测试

```go
func TestKnowledgeTools_Integration(t *testing.T) {
    // 1. 创建 knowledge
    // 2. 查询 resources
    // 3. 更新 knowledge 的 resource
    // 4. 验证更新成功
}
```

### 3. AI 交互测试

使用实际的 Agent 测试 AI 的调用行为:

- 测试快速创建场景(不查询 resources)
- 测试结构化管理场景(先查询再创建)
- 测试更新内容和移动分类
- 测试错误处理和提示

## 部署和配置

### 1. Feature Flag

```go
// 支持通过配置开关控制是否启用 knowledge tools
type AgentConfig struct {
    EnableKnowledgeTools bool `toml:"enable_knowledge_tools"`
}

// 在 agent 初始化时检查
func buildAgent(config AgentConfig) {
    if config.EnableKnowledgeTools {
        options = append(options, NewWithKnowledgeTools())
    }
}
```

### 2. 权限控制

所有 tool 操作都基于实例化时的 `userID` 和 `spaceID`,确保:
- 只能操作属于当前用户的 knowledge
- 只能访问用户有权限的 resources
- 遵循现有的 RBAC 机制

## 性能考虑

### 1. ListUserResources 缓存

```go
// 可以在 tool 实例化时预加载 resources,避免每次调用都查询
type ListUserResourcesTool struct {
    core      *core.Core
    userID    string
    resources []types.Resource // 缓存
}

func NewListUserResourcesTool(core *core.Core, userID string) *ListUserResourcesTool {
    tool := &ListUserResourcesTool{
        core:   core,
        userID: userID,
    }
    // 预加载(可选)
    tool.loadResources()
    return tool
}
```

### 2. 异步处理

- Knowledge 创建后的 summarization 和 embedding 已经是异步的
- UpdateKnowledge 同样使用异步处理,不阻塞 tool 返回

### 3. 批量操作优化(未来)

如果 AI 需要批量创建/更新,可以考虑:
- 提供 `BatchCreateKnowledge` tool
- 或在 tool 描述中引导 AI 逐个调用(当前方案)

## 风险和注意事项

### 1. Resource 误选择风险

**风险**: AI 可能选择不合适的 resource

**缓解措施**:
- 清晰的 resource 描述,帮助 AI 理解语义
- 在 tool 描述中强调"建议先查询 resources"
- 支持后续通过 UpdateKnowledge 修正

### 2. 参数理解偏差

**风险**: AI 可能误解参数含义(如 contentType)

**缓解措施**:
- 参数描述清晰明确
- 提供默认值,降低使用难度
- 在错误提示中包含示例

### 3. 性能问题

**风险**: 频繁创建可能导致系统负载

**缓解措施**:
- 异步处理重计算任务
- 监控 tool 调用频率
- 必要时添加 rate limiting

### 4. 数据一致性

**风险**: Resource 被删除后,相关 knowledge 的状态

**缓解措施**:
- Resource 删除时同步清理 knowledge(已有机制)
- 创建/更新时验证 resource 存在
- 事务保证原子性

## 后续优化方向

### 1. 语义匹配增强

在 CreateKnowledgeTool 中,可以添加可选的语义匹配:
```go
// 分析内容,推荐合适的 resource
recommendedResource := analyzeContentAndRecommendResource(content, availableResources)
// 在返回中提示: "Tip: Based on content analysis, 'projects' might be a better fit."
```

### 2. 批量操作支持

```go
// 新增批量创建 tool
type BatchCreateKnowledgeTool struct {
    // ...
}

// 参数支持数组
type BatchCreateParams struct {
    Items []CreateKnowledgeParams `json:"items"`
}
```

### 3. Template 支持

```go
// 支持从模板创建
type CreateFromTemplateParams struct {
    TemplateID string            `json:"templateId"`
    Variables  map[string]string `json:"variables"`
}
```

### 4. 智能标签建议

```go
// 根据内容自动建议标签
suggestedTags := extractTagsFromContent(content)
// 返回: "Suggested tags: programming, python, tutorial"
```

## 总结

本设计方案通过**智能默认 + 辅助查询**的策略,优雅地解决了 resource 粒度交互的难题:

1. **简单场景**: 用户/AI 无需关心 resource,直接创建即可
2. **复杂场景**: AI 能够通过 ListUserResources 发现并选择合适的分类
3. **灵活调整**: UpdateKnowledgeTool 支持后续修正分类
4. **可扩展性**: 为未来的语义匹配、批量操作等功能预留空间

通过这套 tools,QukaAI 的 AI Agent 将具备完整的 knowledge 管理能力,真正成为用户的"第二大脑"助手。

## 实施检查清单

### 阶段一:核心功能实现
- [ ] 实现 CreateKnowledgeTool
- [ ] 实现 UpdateKnowledgeTool
- [ ] 实现 ListUserResourcesTool
- [ ] 实现 GetKnowledgeTools 函数
- [ ] 添加 WithKnowledgeTools option

### 阶段二:测试
- [ ] 单元测试:CreateKnowledgeTool
- [ ] 单元测试:UpdateKnowledgeTool
- [ ] 单元测试:ListUserResourcesTool
- [ ] 集成测试:完整 CRUD 流程
- [ ] AI 交互测试:验证实际使用场景

### 阶段三:文档和部署
- [ ] API 文档
- [ ] 使用示例
- [ ] 配置说明
- [ ] 部署和监控
