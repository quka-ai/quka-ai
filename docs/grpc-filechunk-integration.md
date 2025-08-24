# gRPC File Chunking Service Integration

## 概述

本文档说明如何将现有的文件分块功能从HTTP服务迁移到新的gRPC文件分块服务。新的gRPC服务提供了更好的性能、类型安全和语义分块功能。

## 变更内容

### 1. 配置更新

简化了配置结构，`ChunkService` 直接包含gRPC配置：

```toml
[chunk_service]
# gRPC configuration (preferred)
enabled = true
address = "localhost:50051"     # gRPC服务地址
timeout = 60                    # 超时时间（秒）

# HTTP fallback configuration (legacy)
http_enabled = true
http_endpoint = "http://localhost:8080/markitdown"  # HTTP后备服务
```

### 2. 新增组件

#### 直接使用生成的gRPC客户端
- **位置**: `pkg/proto/filechunker/filechunker_grpc.pb.go`
- **功能**: 使用protobuf生成的原生gRPC客户端，无需额外封装

#### Proto生成的代码
- **位置**: `pkg/proto/filechunker/`
- **文件**: 
  - `filechunker.proto` - 服务定义
  - `filechunker.pb.go` - 消息类型
  - `filechunker_grpc.pb.go` - gRPC客户端/服务端代码

### 3. 核心逻辑更新

#### ContentTaskProcess 增强
- **文件**: `app/logic/v1/process/content_task.go`
- **新增字段**: `grpcConn` 和 `grpcClient` - 直接使用生成的gRPC客户端
- **新增方法**: `chunkByGRPC()` - 使用gRPC服务进行文件分块
- **资源管理**: `Close()` 方法用于正确关闭gRPC连接
- **智能路由**: 优先使用gRPC服务，如果不可用则回退到HTTP服务

## gRPC服务特性

### Genie分块策略
1. **SLUMBER** - LLM驱动的语义分块（推荐）
2. **RECURSIVE** - 递归分块（降级选项）
3. **TOKEN** - 基于令牌的分块（降级选项）

### 支持的LLM提供商
1. **OpenAI** - GPT系列模型
2. **Gemini** - Google Gemini系列
3. **AUTO** - 自动选择最佳提供商

### 增强功能
- **语义摘要**: 每个分块包含语义摘要
- **关键概念提取**: 自动提取关键概念
- **复杂度评分**: 内容复杂度分析
- **使用统计**: 详细的LLM使用情况跟踪

## 使用说明

### 启用gRPC分块服务

1. **配置gRPC服务**:
   ```toml
   [chunk_service]
   enabled = true
   address = "your-grpc-service:50051"
   timeout = 60
   ```

2. **配置AI模型**:
   确保在AI配置中设置了enhance类型的模型：
   ```toml
   [ai.usage]
   enhance = "your-enhance-model"
   ```

3. **启动服务**:
   重启应用程序，系统会自动检测gRPC配置并初始化客户端。

### 降级机制

如果gRPC服务不可用，系统会自动回退到原有的HTTP服务：
- gRPC连接失败 → 使用HTTP服务
- gRPC分块失败 → 记录错误并重试

### 监控和日志

系统会记录以下信息：
- gRPC客户端初始化状态
- 分块请求和响应详情
- LLM使用统计
- 错误和降级情况

## 示例配置

参考 `config-example-grpc.toml` 文件查看完整的配置示例。

## 故障排除

### 常见问题

1. **gRPC连接失败**
   - 检查服务地址和端口
   - 确认gRPC服务正在运行
   - 检查网络连接

2. **分块失败**
   - 检查AI模型配置
   - 确认API密钥有效
   - 检查文件格式支持

3. **性能问题**
   - 调整超时设置
   - 检查gRPC服务资源使用
   - 考虑缓存优化

### 日志关键字

搜索以下关键字排查问题：
- `gRPC filechunk client`
- `ChunkFile gRPC request`
- `Failed to chunk file via gRPC`
- `gRPC chunking failed`

## 开发注意事项

1. **直接使用生成的客户端**: 无需自定义包装器，直接使用 `pb.NewFileChunkerServiceClient(conn)`
2. **类型转换**: 注意 `types.ContentTask` 和 `sqlstore.ContentTask` 之间的转换
3. **错误处理**: 确保正确处理gRPC错误和降级场景
4. **资源管理**: 使用 `ContentTaskProcess.Close()` 方法正确关闭gRPC连接
5. **配置验证**: 启动时验证gRPC配置的完整性

## 未来优化

1. **连接池**: 实现gRPC连接池以提高性能
2. **重试机制**: 添加智能重试策略
3. **负载均衡**: 支持多个gRPC服务实例
4. **流式处理**: 支持大文件的流式分块
5. **缓存优化**: 改进分块结果缓存机制