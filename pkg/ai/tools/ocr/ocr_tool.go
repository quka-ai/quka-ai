package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/quka-ai/quka-ai/app/core/srv"
)

type OCRTool struct {
	ocr srv.OCRAI
}

func (t *OCRTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "ocr",
		Desc: "从图像中提取文字内容。当用户提到需要识别图片中的文字、扫描文档、读取图片内容时使用此工具。支持 PDF 和常见图片格式（PNG、JPEG、GIF、WEBP、BMP）。支持单个或批量处理多个图片",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"image_urls": {
				Desc:     "图片URL地址列表，可以是单个URL或多个URL",
				Type:     schema.Array,
				Required: true,
			},
		}),
	}, nil
}

type OCRRequest struct {
	ImageURLs []string `json:"image_urls"`
}

func (t *OCRTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args OCRRequest

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	// 验证参数
	if len(args.ImageURLs) == 0 {
		return "", fmt.Errorf("image_urls must contain at least one URL")
	}

	// 批量处理图片
	results, err := t.processBatch(ctx, args.ImageURLs)
	if err != nil {
		return "", err
	}

	// 如果只有一个结果，直接返回
	if len(results) == 1 {
		return results[0], nil
	}

	// 多个结果，合并返回
	var combined strings.Builder
	for i, result := range results {
		if i > 0 {
			combined.WriteString("\n\n---\n\n")
		}
		fmt.Fprintf(&combined, "## Image %d\n\n%s", i+1, result)
	}

	return combined.String(), nil
}

func (t *OCRTool) processBatch(ctx context.Context, imageURLs []string) ([]string, error) {
	results := make([]string, len(imageURLs))
	errors := make([]error, len(imageURLs))

	var wg sync.WaitGroup
	// 限制并发数为 5
	semaphore := make(chan struct{}, 5)

	for i, url := range imageURLs {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			// 下载图片
			fileData, err := downloadFile(ctx, imageURL)
			if err != nil {
				errors[index] = fmt.Errorf("failed to download image %d: %w", index+1, err)
				return
			}

			// 执行 OCR
			result, err := t.ocr.ProcessOCR(ctx, fileData)
			if err != nil {
				errors[index] = fmt.Errorf("failed to process OCR for image %d: %w", index+1, err)
				return
			}

			results[index] = result.MarkdownText
		}(i, url)
	}

	wg.Wait()

	// 检查是否有错误
	var errMsgs []string
	for i, err := range errors {
		if err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("Image %d: %s", i+1, err.Error()))
		}
	}

	if len(errMsgs) > 0 {
		return results, fmt.Errorf("some images failed to process:\n%s", strings.Join(errMsgs, "\n"))
	}

	return results, nil
}

func downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func NewTool(ctx context.Context, ocr srv.OCRAI) (tool.InvokableTool, error) {
	return &OCRTool{
		ocr: ocr,
	}, nil
}
