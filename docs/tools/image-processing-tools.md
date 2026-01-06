# 图像处理工具使用指南

QukaAI 提供了两个强大的图像处理工具：OCR（图像文字提取）和 Vision（图像理解）。这两个工具都支持单个和批量处理模式。

## OCR 工具

OCR 工具用于从图像中提取文字内容，支持 PDF 和常见图片格式（PNG、JPEG、GIF、WEBP、BMP）。

### API 参数

**image_urls** (必需): 图片 URL 地址数组，支持单个或多个 URL

### 单个图片处理

```json
{
  "image_urls": ["https://example.com/document.jpg"]
}
```

### 批量图片处理

```json
{
  "image_urls": [
    "https://example.com/page1.jpg",
    "https://example.com/page2.jpg",
    "https://example.com/page3.jpg"
  ]
}
```

### 特性

- **并发处理**：最多同时处理 5 个图片，提高处理效率
- **错误处理**：即使部分图片处理失败，也会返回成功处理的结果
- **结果合并**：批量处理时，结果会自动合并为一个包含所有内容的 Markdown 文档

### 使用示例

在 Agent 配置中启用 OCR 工具：

```go
options = append(options, NewWithOCRTool(true))
```

## Vision 工具

Vision 工具用于理解和分析图像内容，可以描述图片场景、识别物体、回答关于图片的问题。

### API 参数

- **image_urls** (必需): 图片 URL 地址数组，支持单个或多个 URL
- **question** (可选): 关于图片的问题或分析要求，默认为"请仔细分析这张图片"

### 单个图片分析

```json
{
  "image_urls": ["https://example.com/photo.jpg"],
  "question": "这张图片里有什么？"
}
```

### 批量图片分析

```json
{
  "image_urls": [
    "https://example.com/photo1.jpg",
    "https://example.com/photo2.jpg"
  ],
  "question": "请分析这些图片的内容"
}
```

### 特性

- **并发处理**：最多同时处理 3 个图片（视觉模型资源消耗较大）
- **统一问题**：批量处理时，所有图片都会使用相同的问题进行分析
- **结构化输出**：批量结果会分别标注每张图片的分析结果

### 使用示例

在 Agent 配置中启用 Vision 工具：

```go
options = append(options, NewWithVisionTool(true))
```

## 完整配置示例

```go
// 创建 Agent 配置
config := NewAgentConfig(agentCtx, toolWrapper, core, enableThinking, messages)

// 应用工具选项
options := []AgentOption{
    NewWithOCRTool(true),      // 启用 OCR 工具
    NewWithVisionTool(true),   // 启用 Vision 工具
    NewWithWebSearch(true),    // 启用网络搜索
    NewWithRAG(true),          // 启用 RAG 知识库
}

// 应用所有选项
for _, opt := range options {
    if err := opt.Apply(config); err != nil {
        return err
    }
}
```

## 性能考虑

### OCR 工具
- 并发限制：5 个并发请求
- 适用场景：批量文档扫描、多页 PDF 处理
- 建议批量大小：不超过 20 个图片

### Vision 工具
- 并发限制：3 个并发请求（考虑到视觉模型的资源消耗）
- 适用场景：相册分析、多图对比
- 建议批量大小：不超过 10 个图片

## 错误处理

两个工具都实现了优雅的错误处理机制：

1. **部分失败处理**：如果批量处理中部分图片失败，工具会继续处理其他图片
2. **详细错误信息**：错误消息会明确指出哪些图片处理失败以及失败原因
3. **超时保护**：下载和处理都使用 context 控制超时

## 最佳实践

1. **统一的数组格式**：无论单图还是批量，都使用 `image_urls` 数组，简化 API 调用
2. **合理批量大小**：根据图片大小和复杂度调整批量大小
3. **错误重试**：对于失败的图片，可以单独重新提交处理
4. **资源监控**：监控 AI 模型的资源使用情况，避免过载
5. **成本控制**：批量处理会消耗更多 AI 配额，注意成本控制

## 技术实现

### 并发控制

使用信号量（semaphore）模式控制并发数：

```go
semaphore := make(chan struct{}, maxConcurrency)
// 获取信号量
semaphore <- struct{}{}
defer func() { <-semaphore }() // 释放信号量
```

### 结果聚合

- 单个结果：直接返回处理结果
- 批量结果：使用 Markdown 分隔符组织多个结果

```markdown
## Image 1

[第一张图片的结果]

---

## Image 2

[第二张图片的结果]
```
