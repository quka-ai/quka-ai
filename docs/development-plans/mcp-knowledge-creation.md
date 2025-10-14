# QukaAI MCP 知识创建服务开发计划

## 1. 背景和目标

### 1.1 问题描述
当前 QukaAI 的知识创建功能（`CreateKnowledge`）只能通过 HTTP API 调用，这限制了其在自动化场景中的使用。例如：
- 用户无法在 Claude Code CLI 会话中直接将对话内容保存为知识
- 无法通过命令行工具快速创建记忆
- 缺少与外部工具的标准化集成方式

### 1.2 目标
将 `CreateKnowledge` 功能抽象为 MCP (Model Context Protocol) 服务，使其能够：
- 通过 MCP 协议被 Claude Code CLI 和其他 MCP 客户端调用
- 支持从命令行直接创建知识条目
- 保持与现有 HTTP API 的功能一致性
- 提供良好的用户体验和错误处理

### 1.3 使用场景
- **自动记录对话**：在 Claude Code 会话中使用命令自动保存对话内容为知识
- **快速笔记**：通过命令行工具快速创建文本或 Markdown 笔记
- **工作流集成**：与其他 MCP 工具链集成，实现自动化知识管理

## 2. CreateKnowledge 方法分析

### 2.1 核心流程
基于 [knowledge.go:72-98](cmd/service/handler/knowledge.go#L72-L98) 的分析，核心流程为：

```
1. 解析请求参数（Resource, Content, ContentType, Kind, Async）
2. 获取 spaceID（从上下文中注入）
3. 创建 KnowledgeLogic 实例
4. 根据 Async 参数选择同步/异步处理器
5. 调用 InsertContent/InsertContentAsync 方法
6. 返回创建的知识 ID
```

### 2.2 关键依赖
基于 [knowledge.go:602-682](app/logic/v1/knowledge.go#L602-L682) 的分析：

- **认证信息**：需要用户身份（UserID）和空间标识（SpaceID）
- **内容处理**：
  - Blocks 类型需要解析 EditorJS 格式
  - 内容需要加密存储
  - 支持文件引用处理
- **异步处理**：
  - 知识总结（summarize）
  - 向量嵌入（embedding）
  - 文件上传状态更新
- **过期管理**：根据 resource 配置计算过期时间

### 2.3 输入参数
```go
type CreateKnowledgeRequest struct {
    Resource    string                     `json:"resource"`        // 资源分类
    Content     types.KnowledgeContent     `json:"content"`         // 内容（必需）
    ContentType types.KnowledgeContentType `json:"content_type"`    // 内容类型（必需）
    Kind        string                     `json:"kind"`            // 知识类型
    Async       bool                       `json:"async"`           // 是否异步处理
}
```

### 2.4 输出结果
```go
type CreateKnowledgeResponse struct {
    ID string `json:"id"`  // 创建的知识条目 ID
}
```

## 3. MCP 服务设计

### 3.1 MCP 工具定义
```json
{
  "name": "create_knowledge",
  "description": "Create a new knowledge entry in QukaAI system. The knowledge will be processed asynchronously (summarization and embedding) in the background.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "content": {
        "type": "string",
        "description": "The content of the knowledge (markdown or plain text)"
      },
      "content_type": {
        "type": "string",
        "enum": ["markdown", "blocks"],
        "default": "markdown",
        "description": "Content format type. Default is markdown, use 'blocks' for EditorJS format."
      },
      "kind": {
        "type": "string",
        "enum": ["text", "image", "video", "url"],
        "default": "text",
        "description": "Type of knowledge"
      },
      "title": {
        "type": "string",
        "description": "Optional title for the knowledge (will be auto-generated if not provided)"
      },
      "tags": {
        "type": "array",
        "items": {
          "type": "string"
        },
        "description": "Optional tags for categorization"
      }
    },
    "required": ["content"]
  }
}
```

**注意**:
- 移除了 `resource` 和 `async` 参数（从配置 Header 读取和固定行为）
- 添加了 `title` 和 `tags` 参数以提供更好的灵活性
- SpaceID 和 Resource 从 HTTP Header 获取

### 3.2 技术架构（HTTP 传输）
```
┌─────────────────────────────────┐
│  本地 Claude Code CLI           │
└────────────┬────────────────────┘
             │ HTTPS 请求
             │ JSON-RPC 2.0 over HTTP
             │ Bearer Token 认证
             ↓
┌─────────────────────────────────────────────────────────┐
│  远端 QukaAI 服务器 (https://your-quka.com)             │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Gin HTTP Server (:443)                          │  │
│  │  POST /api/v1/mcp                                │  │
│  │  ├─ Bearer Token 验证 (AccessTokenStore)        │  │
│  │  ├─ SpaceID/Resource 从 Header 提取             │  │
│  │  ├─ JSON-RPC 2.0 协议解析                        │  │
│  │  └─ 路由到 MCP Handler                           │  │
│  └──────────────┬───────────────────────────────────┘  │
│                 │                                       │
│  ┌──────────────▼───────────────────────────────────┐  │
│  │  MCP Module (pkg/mcp)                            │  │
│  │  ├─ HTTP Transport Handler                       │  │
│  │  │  (实现 jsonrpc.Receiver/Sender)               │  │
│  │  ├─ MCP Server (基于 go-sdk)                     │  │
│  │  ├─ create_knowledge Tool Handler                │  │
│  │  └─ Error Handling & Logging                     │  │
│  └──────────────┬───────────────────────────────────┘  │
│                 │                                       │
│  ┌──────────────▼───────────────────────────────────┐  │
│  │  Core Services (共享)                            │  │
│  │  ├─ KnowledgeLogic (InsertContentAsync)         │  │
│  │  ├─ Store Layer (PostgreSQL + pgvector)         │  │
│  │  ├─ AI Processing (Background Workers)          │  │
│  │  └─ Object Storage (S3)                         │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### 3.3 目录结构
```
pkg/
└── mcp/
    ├── server.go              # MCP HTTP 服务入口
    ├── handler.go             # Gin HTTP 处理器
    ├── transport/
    │   └── http.go            # 自定义 HTTP Transport（实现 jsonrpc 接口）
    ├── tools/
    │   ├── knowledge.go       # 知识创建工具实现
    │   └── registry.go        # 工具注册中心
    ├── auth/
    │   └── token.go           # Access Token 验证（调用现有 AuthLogic）
    ├── types/
    │   ├── protocol.go        # MCP 协议类型定义
    │   └── request.go         # 请求/响应结构
    └── config.go              # MCP 配置

cmd/service/
└── router/
    └── mcp.go                 # MCP 路由注册

app/
└── core/
    └── srv.go                 # 添加 MCP HTTP 路由到主服务

docs/
└── development-plans/
    └── mcp-knowledge-creation.md  # 本文档

复用现有模块:
- app/logic/v1/auth.go         # Access Token 验证逻辑 ✅
- app/store/sqlstore/access_token.go  # Token 存储 ✅
- cmd/service/handler/user.go  # Token HTTP API ✅
- pkg/types/access_token.go    # Token 类型定义 ✅
```

### 3.4 技术选型和 HTTP 架构方案

#### 3.4.1 MCP SDK 选择

**选择**: [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) (官方 Go SDK)

**理由**:
1. ✅ **官方维护**: 由 Anthropic 和 Google 协作维护
2. ✅ **功能完整**: 完整实现 MCP 规范
3. ✅ **类型安全**: 完整的 Go 类型系统支持，自动 JSON schema 生成
4. ✅ **可扩展**: 支持自定义 Transport 实现

**实现方式**: 基于 SDK 的 `jsonrpc` 包自定义 HTTP Transport

#### 3.4.2 HTTP 传输架构详细设计

**HTTP 端点**:
- **URL**: `POST /api/v1/mcp`
- **协议**: JSON-RPC 2.0
- **认证**: Bearer Token (HTTP Header)
- **配置传递**: 自定义 HTTP Headers

**HTTP 请求格式**:
```http
POST /api/v1/mcp HTTP/1.1
Host: your-quka.com
Content-Type: application/json
Authorization: Bearer quka_access_1234567890abcdef...
X-Space-ID: 550e8400-e29b-41d4-a716-446655440000
X-Resource: claude-conversations

{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "create_knowledge",
    "arguments": {
      "content": "# 会议记录\n\n今天讨论了 MCP 集成方案...",
      "content_type": "markdown",
      "kind": "text"
    }
  },
  "id": 1
}
```

**HTTP 响应格式**:
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Knowledge created: 660e8400-e29b-41d4-a716-446655440001"
      }
    ]
  },
  "id": 1
}
```

**错误响应**:
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "error": {
    "code": -32000,
    "message": "Invalid access token",
    "data": {
      "error_code": "AUTH_FAILED"
    }
  },
  "id": 1
}
```

#### 3.4.3 自定义 HTTP Transport 实现

基于 MCP Go SDK 的 `jsonrpc` 包实现 HTTP 传输层：

```go
// pkg/mcp/transport/http.go
package transport

import (
    "context"
    "encoding/json"
    "io"
    "net/http"

    "github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// HTTPTransport 实现 MCP 的 HTTP 传输层
type HTTPTransport struct {
    w      http.ResponseWriter
    r      *http.Request
    dec    *json.Decoder
    closed bool
}

func NewHTTPTransport(w http.ResponseWriter, r *http.Request) *HTTPTransport {
    return &HTTPTransport{
        w:   w,
        r:   r,
        dec: json.NewDecoder(r.Body),
    }
}

// Receive 实现 jsonrpc.Receiver 接口
func (t *HTTPTransport) Receive(ctx context.Context) (*jsonrpc.Message, error) {
    if t.closed {
        return nil, io.EOF
    }

    var msg jsonrpc.Message
    if err := t.dec.Decode(&msg); err != nil {
        return nil, err
    }

    t.closed = true // HTTP 单次请求/响应模式
    return &msg, nil
}

// Send 实现 jsonrpc.Sender 接口
func (t *HTTPTransport) Send(ctx context.Context, msg *jsonrpc.Message) error {
    t.w.Header().Set("Content-Type", "application/json")
    return json.NewEncoder(t.w).Encode(msg)
}

// Close 关闭连接
func (t *HTTPTransport) Close() error {
    t.closed = true
    return t.r.Body.Close()
}
```

#### 3.4.4 技术优势

**优点**:
- ✅ **真正的远程服务**: 标准 HTTP 协议，无需 SSH 访问
- ✅ **易于部署**: 集成到现有 HTTP 服务，共享端口和基础设施
- ✅ **可扩展性**: 支持负载均衡、多实例部署、水平扩展
- ✅ **安全性**: HTTPS 加密，标准 Bearer Token 认证
- ✅ **兼容性**: 适合公开 SaaS 服务和自托管场景
- ✅ **监控友好**: 可使用现有 HTTP 监控工具（Prometheus、日志）
- ✅ **简化部署**: 无需配置 SSH 密钥和权限

**实现要点**:
- HTTP Transport 作为 Gin 路由的一个 handler
- 每个 HTTP 请求对应一次 MCP 工具调用
- 认证通过 Bearer Token 验证
- SpaceID 和 Resource 通过 HTTP Header 传递
- 复用现有的 Access Token 验证逻辑

**依赖安装**:
```bash
go get github.com/modelcontextprotocol/go-sdk
```

#### 3.4.5 Claude Code 客户端配置

**好消息**: Claude Code CLI 已支持 Streamable HTTP transport！

**添加 MCP 服务器命令**:
```bash
# 使用 Claude Code CLI 添加 QukaAI MCP 服务器
claude mcp add --transport http quka https://your-quka.com/api/v1/mcp

# 配置会保存到 ~/.config/claude-code/mcp_settings.json
```

**配置文件格式** (`~/.config/claude-code/mcp_settings.json`):
```json
{
  "mcpServers": {
    "quka": {
      "transport": {
        "type": "http",
        "url": "https://your-quka.com/api/v1/mcp"
      }
    }
  }
}
```

**认证配置**:

由于 Claude Code 的 HTTP transport 配置可能不直接支持自定义 Headers，有两种方案：

**方案 A: URL 参数传递** (推荐)
```bash
claude mcp add --transport http quka \
  "https://your-quka.com/api/v1/mcp?token=your-64-char-token&space_id=550e8400-e29b-41d4-a716-446655440000"
```

服务端从 URL 参数读取认证信息，然后设置到 context 中。

**方案 B: 环境变量**
```bash
# 在 shell 配置文件中设置
export QUKA_ACCESS_TOKEN="your-64-char-access-token"
export QUKA_SPACE_ID="550e8400-e29b-41d4-a716-446655440000"
export QUKA_RESOURCE="claude-conversations"

# 然后添加服务器
claude mcp add --transport http quka https://your-quka.com/api/v1/mcp
```

MCP 服务端从环境变量读取（如果客户端传递了环境变量）。

**方案 C: 配置文件扩展** (如果支持自定义 Headers)
如果 Claude Code 支持在配置文件中添加自定义 Headers：
```json
{
  "mcpServers": {
    "quka": {
      "transport": {
        "type": "http",
        "url": "https://your-quka.com/api/v1/mcp",
        "headers": {
          "Authorization": "Bearer your-64-char-access-token",
          "X-Space-ID": "550e8400-e29b-41d4-a716-446655440000",
          "X-Resource": "claude-conversations"
        }
      }
    }
  }
}
```

**推荐实现**: 优先实现方案 A（URL 参数），同时支持方案 C（HTTP Headers），提供最大的灵活性。

## 4. 实施步骤

### Phase 1: 基础设施准备
**目标**: 准备 HTTP Transport 和 MCP Server 基础

#### 1.1 安装 MCP SDK
- [ ] 安装官方 MCP Go SDK: `go get github.com/modelcontextprotocol/go-sdk`
- [ ] 创建 `pkg/mcp` 目录及子目录
- [ ] 导入必要的包：
  ```go
  import (
      "github.com/modelcontextprotocol/go-sdk/mcp"
      "github.com/modelcontextprotocol/go-sdk/jsonrpc"
  )
  ```

#### 1.2 实现 HTTP Transport
- [ ] 创建 `pkg/mcp/transport/http.go`
- [ ] 实现 `HTTPTransport` 结构体
- [ ] 实现 `jsonrpc.Receiver` 接口 (`Receive` 方法)
- [ ] 实现 `jsonrpc.Sender` 接口 (`Send` 方法)
- [ ] 实现 `Close` 方法
- [ ] 添加错误处理和日志

#### 1.3 确认现有 Access Token 系统
- [ ] 确认现有 `quka_access_token` 表结构满足需求
- [ ] 确认 AccessTokenStore 接口可用：
  - `GetAccessToken(appid, token)` - 验证 token
  - Token 格式：64字符随机字符串
- [ ] 无需数据库迁移（复用现有表）

### Phase 2: MCP 认证和上下文
**目标**: 实现基于 Bearer Token 的认证机制

#### 2.1 MCP 认证实现
- [ ] 实现 `pkg/mcp/auth/token.go` 中的验证逻辑
- [ ] 支持多种认证方式（优先级从高到低）：
  - 方式 1: URL 参数 (`?token=xxx&space_id=xxx&resource=xxx`)
  - 方式 2: HTTP Header (`Authorization: Bearer {token}`, `X-Space-ID`, `X-Resource`)
  - 方式 3: 环境变量（如果客户端传递）
- [ ] 调用现有的 `AuthLogic.GetAccessTokenDetail(appid, token)` 验证 token
- [ ] 验证 token 有效性（未过期）
- [ ] 从 token 中提取 userID 和 appid
- [ ] 验证用户对 spaceID 的访问权限（通过 UserSpaceStore）
- [ ] 实现错误处理和日志记录

#### 2.2 用户上下文构建
- [ ] 基于验证后的 token 构建请求上下文
- [ ] 注入 userID, appid, spaceID, resource 到 context
- [ ] 复用现有的 UserInfo 和权限验证机制

### Phase 3: MCP 服务核心实现
**目标**: 实现 MCP Server 和 create_knowledge 工具

#### 3.1 MCP Server 实现
- [ ] 实现 `pkg/mcp/server.go` - 基于官方 SDK 的服务器
  ```go
  func NewMCPServer(core *core.Core) *mcp.Server {
      server := mcp.NewServer(&mcp.Implementation{
          Name:    "quka-mcp",
          Version: "v1.0.0",
      }, nil)

      // 注册工具
      tools.RegisterTools(server, core)

      return server
  }
  ```
- [ ] 实现请求处理流程（HTTP -> Transport -> MCP Server -> Tool）
- [ ] 添加请求日志和错误处理包装

#### 3.2 Gin HTTP Handler 实现
- [ ] 实现 `pkg/mcp/handler.go` - Gin HTTP 处理器
  ```go
  func MCPHandler(core *core.Core) gin.HandlerFunc {
      server := NewMCPServer(core)

      return func(c *gin.Context) {
          // 1. 认证检查
          if err := auth.ValidateRequest(c, core); err != nil {
              c.JSON(401, gin.H{"error": err.Error()})
              return
          }

          // 2. 创建 HTTP Transport
          transport := transport.NewHTTPTransport(c.Writer, c.Request)
          defer transport.Close()

          // 3. 处理 MCP 请求
          if err := server.Handle(c.Request.Context(), transport); err != nil {
              // 错误已通过 transport 返回
              return
          }
      }
  }
  ```
- [ ] 处理 HTTP 请求/响应流程
- [ ] 错误处理和日志记录

#### 3.3 create_knowledge 工具实现
- [ ] 实现 `pkg/mcp/tools/knowledge.go`
- [ ] 定义输入/输出结构（使用 jsonschema tags）
  ```go
  type CreateKnowledgeInput struct {
      Content     string   `json:"content" jsonschema:"required,description=The content of the knowledge"`
      ContentType string   `json:"content_type,omitempty" jsonschema:"enum=markdown,enum=blocks"`
      Kind        string   `json:"kind,omitempty" jsonschema:"enum=text,enum=image,enum=video,enum=url"`
      Title       string   `json:"title,omitempty"`
      Tags        []string `json:"tags,omitempty"`
  }
  ```
- [ ] 实现工具处理函数
  ```go
  func HandleCreateKnowledge(
      ctx context.Context,
      req *mcp.CallToolRequest,
      args CreateKnowledgeInput,
  ) (*mcp.CallToolResult, CreateKnowledgeOutput, error) {
      // 从 context 获取认证信息
      userCtx := GetUserContext(ctx)

      // 调用 KnowledgeLogic.InsertContentAsync
      id, err := logic.InsertContentAsync(...)

      return &mcp.CallToolResult{...}, CreateKnowledgeOutput{...}, nil
  }
  ```
- [ ] 调用现有的 `KnowledgeLogic.InsertContentAsync`
- [ ] 错误处理和国际化消息

#### 3.4 工具注册系统
- [ ] 实现 `pkg/mcp/tools/registry.go`
  ```go
  func RegisterTools(server *mcp.Server, core *core.Core) {
      mcp.AddTool(server, &mcp.Tool{
          Name:        "create_knowledge",
          Description: "Create a new knowledge entry",
      }, NewCreateKnowledgeHandler(core))
  }
  ```
- [ ] 支持动态添加新工具

#### 3.5 集成到主 HTTP 服务
- [ ] 在 `cmd/service/router/` 中添加 MCP 路由
  ```go
  func RegisterMCPRoutes(r *gin.Engine, core *core.Core) {
      r.POST("/api/v1/mcp", mcp.MCPHandler(core))
  }
  ```
- [ ] 在 `app/core/srv.go` 中注册路由
- [ ] 添加配置开关（可选）

### Phase 4: 测试和文档
**目标**: 确保质量和可用性

#### 4.1 单元测试
- [ ] HTTP Transport 测试
- [ ] Access Token 验证测试
- [ ] MCP Server 协议测试
- [ ] create_knowledge 工具测试
- [ ] 覆盖率 > 80%

#### 4.2 集成测试
- [ ] 端到端 HTTP 请求测试
- [ ] 使用 curl/Postman 测试 MCP 端点
- [ ] 错误场景测试（无效 token、过期 token、权限不足等）
- [ ] 并发请求测试

#### 4.3 用户文档编写
- [ ] 用户指南：如何创建和管理 Access Token
- [ ] API 文档：HTTP 端点和请求格式
- [ ] 配置指南：如何配置客户端（预留）
- [ ] 使用示例：常见使用场景
- [ ] 故障排查指南：常见问题和解决方案

#### 4.4 开发者文档
- [ ] MCP HTTP 协议实现说明
- [ ] 如何添加新的 MCP 工具
- [ ] 架构设计文档

### Phase 5: 监控和优化
**目标**: 提供生产监控和性能优化

#### 5.1 前端用户界面 ✅
**无需额外开发**，现有 UI 已支持 Access Token 管理

#### 5.2 监控和指标
- [ ] Prometheus 指标：
  - `quka_mcp_requests_total` - MCP 请求总数
  - `quka_mcp_request_duration_seconds` - 请求延迟
  - `quka_mcp_auth_failures_total` - 认证失败次数
  - `quka_mcp_tool_calls_total{tool="create_knowledge"}` - 工具调用次数
- [ ] 日志记录：
  - 所有 MCP 请求和响应
  - Token 使用记录
  - 错误和异常
- [ ] 性能追踪和优化

#### 5.3 部署和运维
- [ ] 更新 Docker 镜像构建脚本
- [ ] 更新部署文档
- [ ] 配置示例更新
- [ ] 负载测试和性能调优

## 5. 关键考虑点

### 5.1 认证策略 ✅
**问题**: MCP 客户端如何安全地认证？

**确认方案**: 使用 Bearer Token 方式
- 用户登录 QukaAI 平台后，在个人中心创建 Access Token
- Token 通过 HTTP Header `Authorization: Bearer {token}` 传递
- 支持 Token 过期、刷新和撤销功能

### 5.2 SpaceID 和 Resource 处理 ✅
**问题**: 如何确定用户的目标空间和资源分类？

**确认方案**: 通过 HTTP Header 传递
- **SpaceID**: 通过 `X-Space-ID` Header 指定
- **Resource**: 通过 `X-Resource` Header 指定，默认 "knowledge"
- 用户可以在不同的 HTTP 请求中指定不同的空间

### 5.3 内容格式转换 ✅
**问题**: Claude Code 输出的内容如何适配到知识系统？

**确认方案**:
- **默认格式**: Markdown (`content_type: "markdown"`)
- **动态调整**: 用户可在调用时指定 `content_type: "blocks"` 切换到 EditorJS 格式

### 5.4 异步处理 ✅
**问题**: MCP 调用是否等待知识处理完成？

**确认方案**: 固定使用异步处理模式
- 调用后立即返回知识 ID
- 后台自动完成总结和向量化
- 用户可通过 QukaAI Web UI 查看处理进度

### 5.5 部署方式 ✅
**问题**: MCP 服务如何部署？

**确认方案**: 集成到主 HTTP 服务
- MCP 作为 HTTP API 的一个路由端点（`/api/v1/mcp`）
- 与现有 HTTP API 共享端口和基础设施
- 无需独立进程或服务

### 5.6 错误恢复
**问题**: 网络中断或服务重启时如何处理？

**方案**:
- HTTP 协议天然支持重试
- 客户端实现重试机制
- 服务端幂等性保证
- 详细的错误码和消息

## 6. 配置和使用示例

### 6.1 QukaAI 服务端配置
```toml
# config.toml (现有配置即可，MCP 通过 HTTP 端点暴露，无需专属配置)

[database]
host = "localhost"
# ...其他现有配置

[ai]
# ...AI 配置

# MCP 通过 HTTP 路由自动启用，无需额外配置
```

### 6.2 创建 Access Token

#### 步骤 1: 在 QukaAI Web UI 中创建 Access Token
1. 登录 QukaAI
2. 进入"个人中心"
3. 点击"Access Token"标签
4. 点击"创建新 Token"
5. 输入用途描述（如 "MCP - Claude Code"）
6. 复制生成的 Token（**仅显示一次，请妥善保存**）

#### 步骤 2: 获取 Space ID
- 在 QukaAI Web UI 的 URL 中查看（如 `https://your-quka.com/space/550e8400-e29b-41d4-a716-446655440000`）
- 或通过 API `GET /api/v1/user/spaces` 获取所有空间列表

### 6.3 HTTP API 调用示例

#### 示例 1: 使用 curl 调用 MCP 端点

**Initialize 请求**:
```bash
curl -X POST https://your-quka.com/api/v1/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-64-char-access-token" \
  -H "X-Space-ID: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "jsonrpc": "2.0",
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
      }
    },
    "id": 1
  }'
```

**List Tools 请求**:
```bash
curl -X POST https://your-quka.com/api/v1/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-64-char-access-token" \
  -H "X-Space-ID: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/list",
    "params": {},
    "id": 2
  }'
```

**Create Knowledge 请求**:
```bash
curl -X POST https://your-quka.com/api/v1/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-64-char-access-token" \
  -H "X-Space-ID: 550e8400-e29b-41d4-a716-446655440000" \
  -H "X-Resource: claude-conversations" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "create_knowledge",
      "arguments": {
        "content": "# 会议记录\n\n今天讨论了 MCP HTTP 集成方案...",
        "content_type": "markdown",
        "kind": "text",
        "title": "MCP 集成会议记录",
        "tags": ["mcp", "meeting"]
      }
    },
    "id": 3
  }'
```

#### 示例 2: 使用 Python 调用

```python
import requests

url = "https://your-quka.com/api/v1/mcp"
headers = {
    "Content-Type": "application/json",
    "Authorization": "Bearer your-64-char-access-token",
    "X-Space-ID": "550e8400-e29b-41d4-a716-446655440000",
    "X-Resource": "claude-conversations"
}

payload = {
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
        "name": "create_knowledge",
        "arguments": {
            "content": "# Python 调用示例\n\n这是通过 Python 创建的知识...",
            "content_type": "markdown",
            "kind": "text"
        }
    },
    "id": 1
}

response = requests.post(url, json=payload, headers=headers)
print(response.json())
```

#### 示例 3: 使用 Go 调用

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
)

func main() {
    url := "https://your-quka.com/api/v1/mcp"

    payload := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name": "create_knowledge",
            "arguments": map[string]interface{}{
                "content":      "# Go 调用示例\n\n这是通过 Go 创建的知识...",
                "content_type": "markdown",
                "kind":         "text",
            },
        },
        "id": 1,
    }

    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer your-64-char-access-token")
    req.Header.Set("X-Space-ID", "550e8400-e29b-41d4-a716-446655440000")

    client := &http.Client{}
    resp, _ := client.Do(req)
    defer resp.Body.Close()

    // 处理响应...
}
```

## 7. 时间估算

| 阶段 | 任务 | 预计时间 | 优先级 |
|------|------|----------|---------|
| Phase 1 | 基础设施准备（SDK、Transport、Auth） | 1-1.5 天 | 高 |
| Phase 2 | MCP 认证和上下文 | 0.5-1 天 | 高 |
| Phase 3 | MCP 服务核心实现 | 2-3 天 | 高 |
| Phase 4 | 测试和文档 | 2-2.5 天 | 高 |
| Phase 5 | 监控和优化 | 1 天 | 中 |
| **总计** | | **6.5-9 天** | |

**关键里程碑**:
- Day 1-2: Phase 1-2 完成，HTTP Transport 和认证可用
- Day 3-5: Phase 3 完成，MCP 服务和工具可用
- Day 6-8: Phase 4 完成，测试通过
- Day 9: Phase 5 完成，生产就绪

**相比 stdio 方案的优势**: HTTP 方案虽然需要自定义 Transport，但无需 SSH 配置，更适合生产环境

## 8. 风险和挑战

### 8.1 技术风险
- **MCP 协议兼容性**: 需要严格遵循 MCP 规范，确保与各类客户端兼容
- **HTTP Transport 实现**: 需要正确实现 jsonrpc 接口
- **并发处理**: 需要处理多个并发 HTTP 请求

### 8.2 安全风险
- **Token 泄露**: 需要安全存储和传输认证信息
- **权限绕过**: 确保 MCP 路径不会绕过现有权限检查
- **注入攻击**: 严格验证和清理用户输入
- **CORS 配置**: 如果支持浏览器访问，需要正确配置 CORS

### 8.3 兼容性风险
- **现有功能影响**: 确保不影响现有 HTTP API
- **Claude Code 支持**: Claude Code CLI 可能尚未支持 HTTP transport，需要临时方案或等待官方支持

## 9. 后续扩展

### 9.1 其他 MCP 工具
- `query_knowledge`: 检索知识
- `update_knowledge`: 更新知识
- `delete_knowledge`: 删除知识
- `list_knowledge`: 列出知识

### 9.2 高级特性
- 批量操作支持
- 流式内容处理（SSE）
- 多语言客户端 SDK
- 支持更多内容类型（图片、音频等）
- OAuth 认证支持（替代 Bearer Token）

### 9.3 性能优化
- HTTP/2 支持
- 连接池和复用
- 响应缓存
- 负载均衡

## 10. 参考资料

### 10.1 内部文档
- [app/logic/v1/knowledge.go](app/logic/v1/knowledge.go) - 知识逻辑层实现
- [cmd/service/handler/knowledge.go](cmd/service/handler/knowledge.go) - HTTP 处理器
- [pkg/types/knowledge.go](pkg/types/knowledge.go) - 数据类型定义

### 10.2 外部规范和资源
- [MCP 协议规范](https://modelcontextprotocol.io/)
- [MCP Specification - Transports](https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/)
- [MCP Go SDK (官方)](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Go SDK 文档](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp)
- [JSON-RPC 2.0 规范](https://www.jsonrpc.org/specification)

## 11. 技术决策确认 ✅

基于与用户的讨论，以下技术方案已确认：

| 问题 | 确认方案 | 理由 |
|------|---------|------|
| **传输方式** | HTTP + JSON-RPC（唯一方案） | 真正的远程服务，无需 SSH |
| **认证方式** | Bearer Token (复用现有 Access Token) | 简单、安全、易管理 |
| **SpaceID 来源** | HTTP Header (`X-Space-ID`) | 灵活、明确、RESTful |
| **Resource 来源** | HTTP Header (`X-Resource`) | 与 SpaceID 一致的管理方式 |
| **默认内容格式** | Markdown（可动态切换 blocks） | 符合 CLI 使用习惯 |
| **异步处理** | 固定使用异步（无需用户选择） | 简化参数、提升响应速度 |
| **部署方式** | 集成到主 HTTP 服务 | 共享基础设施、简化部署 |
| **MCP SDK** | 官方 Go SDK + 自定义 HTTP Transport | 官方支持 + 灵活扩展 |

## 12. 复用现有 Access Token 系统 ✅

### 12.1 现有数据库表
**无需创建新表**，直接使用现有的 `quka_access_token` 表：

```sql
-- 现有表结构（无需修改）
CREATE TABLE IF NOT EXISTS quka_access_token (
    id SERIAL PRIMARY KEY,
    appid VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    token VARCHAR(255) NOT NULL,
    version VARCHAR(10) NOT NULL,
    info TEXT,  -- 用于标识 token 用途，如 "MCP - Claude Code"
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quka_access_token_appid_token ON quka_access_token (appid, token);
```

### 12.2 Token 生成和格式
**使用现有的 token 生成逻辑**：
- **生成方式**: `utils.RandomStr(64)` - 64 字符随机字符串
- **存储方式**: 明文存储（已有的安全机制）
- **过期时间**: 用户创建时可选择，默认 999 年（长期有效）
- **版本**: `v1` (DEFAULT_ACCESS_TOKEN_VERSION)

## 13. 复用现有 API 接口 ✅

### 13.1 验证 Access Token（MCP 使用）
MCP 服务内部调用 `AuthLogic.GetAccessTokenDetail(appid, token)` 进行验证：
- **实现位置**: [auth.go:33-40](app/logic/v1/auth.go#L33-L40)
- **返回**: `*types.AccessToken` 包含 userID, appid, 过期时间等信息

### 13.2 创建 Access Token（用户使用）
用户通过现有 HTTP API 创建 token：
- **端点**: `POST /api/v1/user/access-token`
- **实现位置**: [user.go:77-147](cmd/service/handler/user.go#L77-L147)

## 14. 代码实现示例

### 14.1 HTTP Transport 完整实现

```go
// pkg/mcp/transport/http.go
package transport

import (
    "context"
    "encoding/json"
    "io"
    "net/http"

    "github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// HTTPTransport 实现 MCP 的 HTTP 传输层
type HTTPTransport struct {
    w      http.ResponseWriter
    r      *http.Request
    dec    *json.Decoder
    closed bool
}

func NewHTTPTransport(w http.ResponseWriter, r *http.Request) *HTTPTransport {
    return &HTTPTransport{
        w:   w,
        r:   r,
        dec: json.NewDecoder(r.Body),
    }
}

// Receive 实现 jsonrpc.Receiver 接口
func (t *HTTPTransport) Receive(ctx context.Context) (*jsonrpc.Message, error) {
    if t.closed {
        return nil, io.EOF
    }

    var msg jsonrpc.Message
    if err := t.dec.Decode(&msg); err != nil {
        return nil, err
    }

    t.closed = true // HTTP 单次请求/响应模式
    return &msg, nil
}

// Send 实现 jsonrpc.Sender 接口
func (t *HTTPTransport) Send(ctx context.Context, msg *jsonrpc.Message) error {
    t.w.Header().Set("Content-Type", "application/json")
    return json.NewEncoder(t.w).Encode(msg)
}

// Close 关闭连接
func (t *HTTPTransport) Close() error {
    t.closed = true
    return t.r.Body.Close()
}
```

### 14.2 MCP Server 和 Handler 实现

```go
// pkg/mcp/server.go
package mcp

import (
    "context"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/quka-ai/quka-ai/app/core"
    "github.com/quka-ai/quka-ai/pkg/mcp/tools"
)

type MCPServer struct {
    server *mcp.Server
    core   *core.Core
}

func NewMCPServer(core *core.Core) *MCPServer {
    // 创建 MCP 服务器
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "quka-mcp",
        Version: "v1.0.0",
    }, nil)

    // 注册所有工具
    tools.RegisterTools(server, core)

    return &MCPServer{
        server: server,
        core:   core,
    }
}

func (s *MCPServer) Handle(ctx context.Context, transport *transport.HTTPTransport) error {
    return s.server.Handle(ctx, transport)
}
```

```go
// pkg/mcp/handler.go
package mcp

import (
    "github.com/gin-gonic/gin"
    "github.com/quka-ai/quka-ai/app/core"
    "github.com/quka-ai/quka-ai/pkg/mcp/auth"
    "github.com/quka-ai/quka-ai/pkg/mcp/transport"
)

func MCPHandler(core *core.Core) gin.HandlerFunc {
    mcpServer := NewMCPServer(core)

    return func(c *gin.Context) {
        // 1. 认证检查（从 Header 提取 Token, SpaceID, Resource）
        userCtx, err := auth.ValidateRequest(c, core)
        if err != nil {
            c.JSON(401, gin.H{
                "jsonrpc": "2.0",
                "error": map[string]interface{}{
                    "code":    -32000,
                    "message": err.Error(),
                },
                "id": nil,
            })
            return
        }

        // 2. 将用户上下文注入到 Gin Context
        c.Set("user_context", userCtx)

        // 3. 创建 HTTP Transport
        httpTransport := transport.NewHTTPTransport(c.Writer, c.Request)
        defer httpTransport.Close()

        // 4. 处理 MCP 请求
        if err := mcpServer.Handle(c.Request.Context(), httpTransport); err != nil {
            // 错误已通过 transport 返回给客户端
            return
        }
    }
}
```

### 14.3 认证中间件实现

```go
// pkg/mcp/auth/token.go
package auth

import (
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/quka-ai/quka-ai/app/core"
    v1 "github.com/quka-ai/quka-ai/app/logic/v1"
)

type UserContext struct {
    UserID   string
    Appid    string
    SpaceID  string
    Resource string
}

func ValidateRequest(c *gin.Context, core *core.Core) (*UserContext, error) {
    var token, spaceID, resource string

    // 认证方式优先级：URL 参数 > HTTP Header > 环境变量

    // 方式 1: 从 URL 参数提取
    token = c.Query("token")
    spaceID = c.Query("space_id")
    resource = c.Query("resource")

    // 方式 2: 从 HTTP Header 提取（如果 URL 参数为空）
    if token == "" {
        authHeader := c.GetHeader("Authorization")
        if authHeader != "" {
            parts := strings.SplitN(authHeader, " ", 2)
            if len(parts) == 2 && parts[0] == "Bearer" {
                token = parts[1]
            }
        }
    }

    if spaceID == "" {
        spaceID = c.GetHeader("X-Space-ID")
    }

    if resource == "" {
        resource = c.GetHeader("X-Resource")
    }

    // 方式 3: 从环境变量提取（如果前两种方式都为空）
    if token == "" {
        token = os.Getenv("QUKA_ACCESS_TOKEN")
    }
    if spaceID == "" {
        spaceID = os.Getenv("QUKA_SPACE_ID")
    }
    if resource == "" {
        resource = os.Getenv("QUKA_RESOURCE")
    }

    // 验证必需参数
    if token == "" {
        return nil, fmt.Errorf("missing access token (provide via URL param, Authorization header, or env var)")
    }
    if spaceID == "" {
        return nil, fmt.Errorf("missing space_id (provide via URL param, X-Space-ID header, or env var)")
    }
    if resource == "" {
        resource = "knowledge" // 默认值
    }

    // 验证 token
    appid := core.Cfg().Appid
    ctx := c.Request.Context()

    authLogic := v1.NewAuthLogic(ctx, core)
    accessToken, err := authLogic.GetAccessTokenDetail(appid, token)
    if err != nil {
        return nil, fmt.Errorf("invalid access token: %w", err)
    }

    // 检查过期
    if accessToken.ExpiresAt > 0 && accessToken.ExpiresAt < time.Now().Unix() {
        return nil, fmt.Errorf("access token expired")
    }

    // 验证用户对空间的访问权限
    // TODO: 调用 UserSpaceStore 验证用户是否有权访问该空间

    return &UserContext{
        UserID:   accessToken.UserID,
        Appid:    accessToken.Appid,
        SpaceID:  spaceID,
        Resource: resource,
    }, nil
}
```

### 14.4 create_knowledge 工具完整实现

```go
// pkg/mcp/tools/knowledge.go
package tools

import (
    "context"
    "encoding/json"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/quka-ai/quka-ai/app/core"
    v1 "github.com/quka-ai/quka-ai/app/logic/v1"
    "github.com/quka-ai/quka-ai/pkg/mcp/auth"
    "github.com/quka-ai/quka-ai/pkg/types"
)

// 输入结构（SDK 会自动生成 JSON schema）
type CreateKnowledgeInput struct {
    Content     string   `json:"content" jsonschema:"required,description=The content of the knowledge (markdown or plain text)"`
    ContentType string   `json:"content_type,omitempty" jsonschema:"enum=markdown,enum=blocks,default=markdown,description=Content format type"`
    Kind        string   `json:"kind,omitempty" jsonschema:"enum=text,enum=image,enum=video,enum=url,default=text,description=Type of knowledge"`
    Title       string   `json:"title,omitempty" jsonschema:"description=Optional title for the knowledge"`
    Tags        []string `json:"tags,omitempty" jsonschema:"description=Optional tags for categorization"`
}

// 输出结构
type CreateKnowledgeOutput struct {
    ID      string `json:"id"`
    Status  string `json:"status"`
    Message string `json:"message"`
}

// 工具处理器
type CreateKnowledgeHandler struct {
    core *core.Core
}

func NewCreateKnowledgeHandler(core *core.Core) *CreateKnowledgeHandler {
    return &CreateKnowledgeHandler{core: core}
}

func (h *CreateKnowledgeHandler) Handle(
    ctx context.Context,
    req *mcp.CallToolRequest,
    args CreateKnowledgeInput,
) (*mcp.CallToolResult, CreateKnowledgeOutput, error) {
    // 从 context 获取认证信息（由 auth middleware 注入）
    userCtx := ctx.Value("user_context").(*auth.UserContext)

    // 转换 content type
    contentType := types.StringToKnowledgeContentType(args.ContentType)
    if contentType == types.KNOWLEDGE_CONTENT_TYPE_UNKNOWN {
        contentType = types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN
    }

    // 准备内容
    content := types.KnowledgeContent(args.Content)

    // 调用 KnowledgeLogic 创建知识
    logic := v1.NewKnowledgeLogic(ctx, h.core)
    id, err := logic.InsertContentAsync(
        userCtx.SpaceID,
        userCtx.Resource,
        types.KindNewFromString(args.Kind),
        content,
        contentType,
    )
    if err != nil {
        return nil, CreateKnowledgeOutput{}, err
    }

    // 返回结果
    output := CreateKnowledgeOutput{
        ID:      id,
        Status:  "processing",
        Message: "Knowledge created successfully, processing in background",
    }

    return &mcp.CallToolResult{
        Content: []interface{}{
            map[string]interface{}{
                "type": "text",
                "text": "Knowledge created: " + id,
            },
        },
    }, output, nil
}

// 注册工具
func RegisterCreateKnowledgeTool(server *mcp.Server, core *core.Core) {
    handler := NewCreateKnowledgeHandler(core)

    mcp.AddTool(server, &mcp.Tool{
        Name:        "create_knowledge",
        Description: "Create a new knowledge entry in QukaAI. The knowledge will be processed asynchronously.",
    }, handler.Handle)
}
```

```go
// pkg/mcp/tools/registry.go
package tools

import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/quka-ai/quka-ai/app/core"
)

func RegisterTools(server *mcp.Server, core *core.Core) {
    // 注册 create_knowledge 工具
    RegisterCreateKnowledgeTool(server, core)

    // 未来可添加更多工具
    // RegisterQueryKnowledgeTool(server, core)
    // RegisterUpdateKnowledgeTool(server, core)
}
```

### 14.5 路由注册

```go
// cmd/service/router/mcp.go
package router

import (
    "github.com/gin-gonic/gin"
    "github.com/quka-ai/quka-ai/app/core"
    "github.com/quka-ai/quka-ai/pkg/mcp"
)

func RegisterMCPRoutes(r *gin.Engine, core *core.Core) {
    r.POST("/api/v1/mcp", mcp.MCPHandler(core))
}
```

```go
// app/core/srv.go (添加到现有代码)
func (s *Server) setupRoutes() {
    // ... 现有路由

    // 添加 MCP 路由
    router.RegisterMCPRoutes(s.engine, s.core)
}
```

## 15. 后续步骤

### 15.1 立即行动
- [ ] Review 并确认最终方案（HTTP only）
- [ ] 安装 MCP Go SDK: `go get github.com/modelcontextprotocol/go-sdk`
- [ ] 准备开发环境
- [ ] 创建开发分支 `feat/mcp-knowledge-creation-http`

### 15.2 第一周目标（Day 1-5）
- [ ] 完成 Phase 1-3（HTTP Transport + 核心功能）
- [ ] 完成基础测试（curl/Postman 测试）
- [ ] 编写 API 文档

### 15.3 第二周目标（Day 6-9）
- [ ] 完成 Phase 4-5（测试和监控）
- [ ] 代码 review
- [ ] 准备发布

### 15.4 未来计划
- [ ] ✅ Claude Code CLI 已支持 Streamable HTTP transport - 无需等待！
- [ ] 添加更多 MCP 工具（query、update、delete 等）
- [ ] 支持更多认证方式（OAuth、API Key 等）
- [ ] 实现工具的批量操作
- [ ] 添加速率限制和配额管理

---

**文档版本**: v5.1 (HTTP + Streamable HTTP - 最终确认版)
**创建时间**: 2025-10-13
**更新时间**: 2025-10-13
**作者**: Claude (AI Assistant)
**状态**: ✅ 已确认，待实施
**技术栈**: Go 1.23.1+ | MCP Go SDK (官方) | HTTP + JSON-RPC (Streamable) | Gin | Claude Code CLI

## 重要变更记录

### v5.0 (2025-10-13) - HTTP Only 架构（最终确认）
- ✅ **移除所有 SSH 方案**，严格使用 HTTP 传输
- ✅ 基于 MCP Go SDK 的 jsonrpc 包自定义 HTTP Transport
- ✅ 集成到现有 Gin HTTP 服务（`/api/v1/mcp` 端点）
- ✅ 使用 Bearer Token 认证（复用现有 Access Token）
- ✅ 通过 HTTP Header 传递 SpaceID 和 Resource
- ✅ 更新所有配置示例和代码示例为 HTTP 方式
- ✅ 添加完整的 curl、Python、Go 调用示例

### v5.1 (2025-10-13) - Claude Code CLI 支持确认
- ✅ **确认 Claude Code CLI 已支持 Streamable HTTP transport**
- ✅ 添加 `claude mcp add --transport http` 命令示例
- ✅ 实现多种认证方式：URL 参数、HTTP Header、环境变量
- ✅ 推荐使用 URL 参数方式（最简单）
- ✅ 更新认证代码以支持多种认证来源

### v4.0 (2025-10-13) - SSH 方案（已废弃）
- ❌ 使用 SSH + stdio 方案（用户明确拒绝）

### v3.0 (2025-10-13) - 添加技术选型
- 选择官方 MCP Go SDK

### v2.0 (2025-10-13) - 复用现有系统
- 复用现有 quka_access_token 表和 API

### v1.0 (2025-10-13) - 初版
- 基础架构设计

## 最终确认方案总结

| 方面 | 方案 | 说明 |
|------|------|------|
| **MCP SDK** | 官方 Go SDK | 由 Anthropic + Google 维护 |
| **认证** | Bearer Token (复用现有 Access Token) | 64字符随机字符串，无需新建表 |
| **传输方式** | HTTP + JSON-RPC (唯一方案) | 真正的远程服务，无需 SSH |
| **配置方式** | HTTP Headers | Authorization + X-Space-ID + X-Resource |
| **Transport 实现** | 自定义 HTTPTransport | 实现 jsonrpc.Receiver/Sender 接口 |
| **部署方式** | 集成到主 HTTP 服务 | Gin 路由端点 `/api/v1/mcp` |
| **内容格式** | Markdown (默认) | 可选 blocks 格式 |
| **处理模式** | 异步 (固定) | 立即返回 ID |

## 适用场景

✅ **HTTP 方案适用于**:
- 所有场景（远程访问、公开 SaaS、自托管）
- 无需 SSH 访问权限
- 标准 HTTP 客户端即可调用
- 支持负载均衡和水平扩展
- 易于监控和调试

❌ **不使用 SSH 方案**:
- 用户明确要求："永远不要使用ssh方案，请使用http方案"
