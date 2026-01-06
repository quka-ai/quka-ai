# Baidu OCR Provider

这是一个百度 OCR API 的 Go 语言客户端实现，用于在 QukaAI 项目中提供 OCR（光学字符识别）功能。

## 功能特性

- 支持图片 OCR 识别
- 支持 PDF 文档 OCR 识别  
- 自动提取文档中的图片
- 返回 Markdown 格式的识别结果
- 支持图片下载功能

## 配置

```go
import "github.com/quka-ai/quka-ai/pkg/ai/baidu"

config := baidu.Config{
    APIURL: "您的百度 OCR API 地址",
    Token:  "您的访问令牌",
}

driver := baidu.New(config)
```

## 使用方式

### 图片 OCR 识别

```go
imageData, err := os.ReadFile("image.jpg")
if err != nil {
    log.Fatal(err)
}

result, err := driver.ProcessImageOCR(context.Background(), imageData)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("标题: %s\n", result.Title)
fmt.Printf("Markdown 内容: %s\n", result.MarkdownText)
fmt.Printf("图片链接: %v\n", result.Images)
```

### PDF OCR 识别

```go
pdfData, err := os.ReadFile("document.pdf")
if err != nil {
    log.Fatal(err)
}

result, err := driver.ProcessPDFOCR(context.Background(), pdfData)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("标题: %s\n", result.Title)
fmt.Printf("Markdown 内容: %s\n", result.MarkdownText)
```

### 下载图片

```go
imageData, err := driver.DownloadImage(context.Background(), "https://example.com/image.jpg")
if err != nil {
    log.Fatal(err)
}

err = os.WriteFile("downloaded_image.jpg", imageData, 0644)
if err != nil {
    log.Fatal(err)
}
```

## API 响应结构

### OCRProcessResult

```go
type OCRProcessResult struct {
    Title        string    `json:"title"`         // 文档标题
    MarkdownText string    `json:"markdown_text"` // Markdown 格式文本
    Images       []string  `json:"images"`        // 图片 URL 列表
    Usage        *OCRUsage `json:"usage"`         // 使用统计
    Model        string    `json:"model"`         // 模型名称
}
```

### OCRUsage

```go
type OCRUsage struct {
    TokensUsed int `json:"tokens_used"` // 使用的令牌数量
}
```

## 文件类型支持

- `fileType: 0` - PDF 文档
- `fileType: 1` - 图片文件（JPG、PNG等）

## 错误处理

所有方法都会返回详细的错误信息，包括：
- API 请求错误
- 网络连接错误  
- JSON 解析错误
- 百度 API 返回的业务错误

## 测试

运行单元测试：

```bash
go test ./pkg/ai/baidu/...
```

## 注意事项

1. 确保您拥有有效的百度 OCR API 访问凭证
2. 大文件处理可能需要较长时间
3. 网络连接不稳定可能影响图片下载功能
4. API 调用频率受百度服务限制

## 接口实现

该包实现了 `OCRProvider` 接口：

```go
type OCRProvider interface {
    ProcessImageOCR(ctx context.Context, imageData []byte) (*OCRProcessResult, error)
    ProcessPDFOCR(ctx context.Context, pdfData []byte) (*OCRProcessResult, error)
    DownloadImage(ctx context.Context, imageURL string) ([]byte, error)
    Lang() string
}
```