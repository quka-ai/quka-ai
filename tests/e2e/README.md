# E2E Tests for QukaAI gRPC FileChunk Service

这个目录包含了QukaAI gRPC文件分块服务的端到端测试。

## 测试结构

- `grpc_filechunk_test.go` - 主要的e2e测试文件
- `go.mod` - Go模块配置文件
- `README.md` - 本说明文档

## 设置测试环境

### 环境变量配置

设置以下环境变量：

```bash
# gRPC服务地址 (必需)
export TEST_GRPC_ADDRESS="localhost:50051"

# OpenAI配置 (必需)
export TEST_OPENAI_API_KEY="sk-your-openai-api-key-here"
export TEST_OPENAI_MODEL="gpt-3.5-turbo"  # 可选，默认值
export TEST_OPENAI_ENDPOINT="https://api.openai.com/v1"  # 可选，默认值
```

### 依赖服务

确保以下服务正在运行：

- **gRPC FileChunk服务**（在指定端口运行，如localhost:50051）

## 运行测试

### 运行所有e2e测试

```bash
cd tests/e2e
go test -v . -run TestGRPCFileChunkService
```

### 运行特定测试

```bash
# 仅测试健康检查
go test -v . -run TestGRPCFileChunkService/HealthCheck

# 测试文本文件分块
go test -v . -run TestGRPCFileChunkService/ChunkTextFile

# 测试并发分块
go test -v . -run TestGRPCFileChunkService/ConcurrentChunking
```

### 使用环境变量运行测试

```bash
TEST_GRPC_ADDRESS="localhost:50051" \
TEST_OPENAI_API_KEY="sk-your-key" \
go test -v . -run TestGRPCFileChunkService
```

## 测试内容

### 1. 健康检查测试 (HealthCheck)
- 验证gRPC服务是否运行正常
- 检查服务版本信息

### 2. 支持的LLM测试 (GetSupportedLLMs)
- 获取服务支持的LLM提供商列表
- 验证OpenAI等主要提供商是否可用

### 3. 文本文件分块测试 (ChunkTextFile)
- 测试长文本内容的智能分块功能
- 验证语义总结和token计数
- 使用真实的OpenAI模型进行处理

### 4. PDF文件分块测试 (ChunkPDF)
- 模拟PDF文档分块处理
- 测试不同文件类型的处理能力

### 5. 并发分块测试 (ConcurrentChunking)
- 测试多个同时请求的处理能力
- 验证并发场景下的服务稳定性

## 故障排除

### 测试跳过
如果看到测试被跳过，检查：
- OpenAI API密钥是否设置（`TEST_OPENAI_API_KEY`）
- gRPC服务是否正在运行

### 连接错误
如果遇到连接错误：
- 确认gRPC服务地址和端口（`TEST_GRPC_ADDRESS`）
- 检查防火墙设置
- 验证gRPC服务是否正在运行

### API调用错误
如果API调用失败：
- 检查OpenAI API密钥是否有效
- 确认网络连接正常
- 验证API配额是否充足

## 最佳实践

1. **API密钥安全**: 不要将真实的API密钥提交到版本控制，使用环境变量
2. **gRPC服务准备**: 确保gRPC FileChunk服务已启动并监听指定端口
3. **并发测试**: 并发测试可能消耗更多API配额，注意控制测试频率

## 扩展测试

可以根据需要添加更多测试场景：

1. **错误处理测试**: 测试各种异常情况
2. **性能测试**: 测试大文件处理能力
3. **多语言测试**: 测试不同语言内容的处理
4. **不同模型测试**: 测试Azure OpenAI、Ollama等不同提供商

## 示例：完整测试命令

```bash
# 设置环境变量并运行测试
export TEST_GRPC_ADDRESS="localhost:50051"
export TEST_OPENAI_API_KEY="sk-your-openai-key-here"
export TEST_OPENAI_MODEL="gpt-3.5-turbo"

# 运行测试
cd tests/e2e
go test -v . -run TestGRPCFileChunkService
```