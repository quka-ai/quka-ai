# Baidu OCR 集成示例

## 集成到 QukaAI 主系统

以下是将 Baidu OCR 提供商集成到 QukaAI 主系统的示例代码：

### 1. 配置文件 (config.toml)

```toml
[ai.baidu]
api_url = "https://aistudio.baidu.com/paddleocr/api/v1/ocr"
token = "your_access_token_here"
```

### 2. 服务初始化

```go
package service

import (
    "github.com/quka-ai/quka-ai/pkg/ai/baidu"
)

type OCRService struct {
    baiduOCR *baidu.Driver
}

func NewOCRService(config baidu.Config) *OCRService {
    return &OCRService{
        baiduOCR: baidu.New(config),
    }
}

func (s *OCRService) ProcessDocument(ctx context.Context, fileData []byte, fileType string) (*baidu.OCRProcessResult, error) {
    switch fileType {
    case "pdf":
        return s.baiduOCR.ProcessPDFOCR(ctx, fileData)
    case "image", "jpg", "jpeg", "png":
        return s.baiduOCR.ProcessImageOCR(ctx, fileData)
    default:
        return nil, fmt.Errorf("unsupported file type: %s", fileType)
    }
}
```

### 3. HTTP 处理器

```go
package handlers

import (
    "io"
    "net/http"
    "path/filepath"
    "strings"
    
    "github.com/gin-gonic/gin"
)

func (h *Handler) HandleOCRUpload(c *gin.Context) {
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file"})
        return
    }
    defer file.Close()

    data, err := io.ReadAll(file)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
        return
    }

    ext := strings.ToLower(filepath.Ext(header.Filename))
    var fileType string
    
    switch ext {
    case ".pdf":
        fileType = "pdf"
    case ".jpg", ".jpeg", ".png", ".bmp":
        fileType = "image"
    default:
        c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file type"})
        return
    }

    result, err := h.ocrService.ProcessDocument(c.Request.Context(), data, fileType)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "title": result.Title,
        "content": result.MarkdownText,
        "images": result.Images,
        "usage": result.Usage,
    })
}
```

### 4. 路由配置

```go
package routes

func SetupOCRRoutes(r *gin.Engine, handler *handlers.Handler) {
    api := r.Group("/api/v1")
    {
        api.POST("/ocr/upload", handler.HandleOCRUpload)
        api.GET("/ocr/download-image", handler.HandleImageDownload)
    }
}
```

### 5. 知识库集成

```go
package knowledge

import (
    "context"
    "fmt"
    
    "github.com/quka-ai/quka-ai/pkg/ai/baidu"
    "github.com/quka-ai/quka-ai/pkg/types"
)

func (s *KnowledgeService) ProcessOCRContent(ctx context.Context, result *baidu.OCRProcessResult, userID string) error {
    // 创建知识条目
    knowledge := &types.Knowledge{
        Title:    result.Title,
        Content:  result.MarkdownText,
        UserID:   userID,
        Resource: "ocr",
        Tags:     extractTagsFromContent(result.MarkdownText),
    }

    // 保存到数据库
    if err := s.store.CreateKnowledge(ctx, knowledge); err != nil {
        return fmt.Errorf("failed to save OCR knowledge: %w", err)
    }

    // 处理图片
    for _, imageURL := range result.Images {
        if err := s.processOCRImage(ctx, knowledge.ID, imageURL); err != nil {
            // 记录错误但继续处理
            slog.Error("Failed to process OCR image", 
                slog.String("error", err.Error()),
                slog.String("image_url", imageURL))
        }
    }

    return nil
}

func (s *KnowledgeService) processOCRImage(ctx context.Context, knowledgeID string, imageURL string) error {
    // 下载图片
    imageData, err := s.baiduOCR.DownloadImage(ctx, imageURL)
    if err != nil {
        return err
    }

    // 保存到文件存储
    filename := fmt.Sprintf("ocr-images/%s-%s.jpg", knowledgeID, generateUUID())
    if err := s.fileService.SaveFile(ctx, filename, imageData); err != nil {
        return err
    }

    // 更新知识条目的图片信息
    return s.store.AddKnowledgeImage(ctx, knowledgeID, filename)
}
```

### 6. 使用示例

```bash
# 上传 PDF 文档进行 OCR 识别
curl -X POST http://localhost:8080/api/v1/ocr/upload \
  -F "file=@document.pdf" \
  -H "Authorization: Bearer your_jwt_token"

# 上传图片进行 OCR 识别
curl -X POST http://localhost:8080/api/v1/ocr/upload \
  -F "file=@image.jpg" \
  -H "Authorization: Bearer your_jwt_token"
```

### 7. 错误处理

```go
package middleware

import (
    "log/slog"
    "net/http"
    
    "github.com/gin-gonic/gin"
)

func OCRErrorHandler() gin.HandlerFunc {
    return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
        slog.Error("OCR processing panic",
            slog.Any("error", recovered),
            slog.String("path", c.Request.URL.Path))
        
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error during OCR processing",
            "code":  "OCR_PROCESSING_ERROR",
        })
    })
}
```

这个集成示例展示了如何将 Baidu OCR 提供商完整地集成到 QukaAI 系统中，包括文件上传、OCR 处理、结果存储和错误处理。