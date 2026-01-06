# E2E Tests for QukaAI Services

这个目录包含了QukaAI各个服务的端到端测试。

## 测试结构

- `grpc_filechunk_test.go` - gRPC文件分块服务的e2e测试
- `baidu_ocr_test.go` - 百度OCR服务的e2e测试
- `test_table_image.jpg` - OCR测试用的示例图片
- `go.mod` - Go模块配置文件
- `.env.example` - 环境变量配置示例
- `README.md` - 本说明文档

## 设置测试环境

### 环境变量配置

复制 `.env.example` 到 `.env` 并配置相应的值：

```bash
cp .env.example .env
# 编辑 .env 文件，填入实际的配置值
```

或者直接设置环境变量：

```bash
# gRPC服务地址 (gRPC测试必需)
export TEST_GRPC_ADDRESS="localhost:35051"

# OpenAI配置 (gRPC文件分块测试必需)
export TEST_OPENAI_API_KEY="sk-your-openai-api-key-here"
export TEST_OPENAI_MODEL="gpt-3.5-turbo"  # 可选，默认值
export TEST_OPENAI_ENDPOINT="https://api.openai.com/v1"  # 可选，默认值

# 百度OCR配置 (百度OCR测试必需)
export QUKA_TEST_BAIDU_OCR_ENDPOINT="https://your-baidu-ocr-api-endpoint"
export QUKA_TEST_BAIDU_OCR_TOKEN="your-baidu-ocr-token"
```

### 依赖服务

确保以下服务正在运行：

- **gRPC FileChunk服务**（在指定端口运行，如localhost:50051）

## 运行测试

### 运行所有e2e测试

```bash
cd tests/e2e
go test -v .
```

### 运行特定测试套件

```bash
# 运行gRPC文件分块测试
go test -v . -run TestGRPCFileChunkService

# 运行百度OCR测试
go test -v . -run TestBaiduOCR
```

### 运行特定测试用例

```bash
# gRPC测试用例
go test -v . -run TestGRPCFileChunkService/HealthCheck
go test -v . -run TestGRPCFileChunkService/ChunkTextFile
go test -v . -run TestGRPCFileChunkService/ConcurrentChunking

# 百度OCR测试用例
go test -v . -run TestBaiduOCR/TestLanguage
go test -v . -run TestBaiduOCR/TestProcessImageOCR
go test -v . -run TestBaiduOCR/TestProcessPDFOCR
go test -v . -run TestConcurrentOCR
```

### 使用环境变量运行测试

```bash
# 运行gRPC测试
TEST_GRPC_ADDRESS="localhost:35051" \
TEST_OPENAI_API_KEY="sk-your-key" \
go test -v . -run TestGRPCFileChunkService

# 运行百度OCR测试
QUKA_TEST_BAIDU_OCR_ENDPOINT="https://your-api-endpoint" \
QUKA_TEST_BAIDU_OCR_TOKEN="your-token" \
go test -v . -run TestBaiduOCR
```

### 运行基准测试

```bash
# 运行百度OCR性能基准测试
go test -bench=. -benchmem -run=^$ -benchtime=3x
```

## 测试内容

### gRPC FileChunk Service 测试

#### 1. 健康检查测试 (HealthCheck)
- 验证gRPC服务是否运行正常
- 检查服务版本信息

#### 2. 支持的LLM测试 (GetSupportedLLMs)
- 获取服务支持的LLM提供商列表
- 验证OpenAI等主要提供商是否可用

#### 3. 文本文件分块测试 (ChunkTextFile)
- 测试长文本内容的智能分块功能
- 验证语义总结和token计数
- 使用真实的OpenAI模型进行处理

#### 4. PDF文件分块测试 (ChunkPDF)
- 模拟PDF文档分块处理
- 测试不同文件类型的处理能力

#### 5. 并发分块测试 (ConcurrentChunking)
- 测试多个同时请求的处理能力
- 验证并发场景下的服务稳定性

### 百度 OCR Service 测试

#### 1. 语言检测测试 (TestLanguage)
- 验证OCR服务返回正确的语言代码

#### 2. 图片OCR测试 (TestProcessImageOCR)
- 使用真实图片文件测试OCR识别
- 验证返回结果的结构（标题、Markdown文本、图片URL等）
- 验证token使用统计

#### 3. PDF OCR测试 (TestProcessPDFOCR)
- 测试PDF文档的OCR处理
- 验证PDF文件类型检测和处理

#### 4. 错误处理测试
- **TestProcessInvalidFile**: 测试无效文件类型的错误处理
- **TestProcessEmptyFile**: 测试空文件的错误处理

#### 5. 并发OCR测试 (TestConcurrentOCR)
- 测试多个并发OCR请求
- 验证并发处理的稳定性和正确性

#### 6. 性能基准测试 (BenchmarkProcessImageOCR)
- 测量OCR处理的平均时间
- 测量内存使用情况

## 故障排除

### 测试跳过
如果看到测试被跳过，检查：
- **gRPC测试**: `TEST_OPENAI_API_KEY` 和 `TEST_GRPC_ADDRESS` 是否设置
- **百度OCR测试**: `QUKA_TEST_BAIDU_OCR_ENDPOINT` 和 `QUKA_TEST_BAIDU_OCR_TOKEN` 是否设置
- 测试所需的服务是否正在运行

### 连接错误
如果遇到连接错误：
- **gRPC测试**: 确认gRPC服务地址和端口（`TEST_GRPC_ADDRESS`），检查服务是否运行
- **百度OCR测试**: 确认API endpoint是否正确，检查网络连接

### API调用错误
如果API调用失败：
- **OpenAI API**: 检查API密钥是否有效，确认网络连接，验证API配额
- **百度OCR API**: 验证token是否有效且未过期，检查API endpoint是否正确

### 测试文件未找到
对于百度OCR测试，如果提示找不到 `test_table_image.jpg`：
```bash
# 确保在正确的目录下运行测试
cd tests/e2e
ls test_table_image.jpg
```

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