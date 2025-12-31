package vision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type VisionTool struct {
	visionModel types.ChatModel
}

func (t *VisionTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "vision",
		Desc: "理解和分析图像内容。当用户需要了解图片中的场景、物体、人物、活动等视觉信息时使用此工具。此工具可以描述图片内容、回答关于图片的问题、识别图片中的元素。支持单个或批量处理多个图片",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"image_urls": {
				Desc:     "图片URL地址列表，可以是单个URL或多个URL",
				Type:     schema.Array,
				Required: true,
			},
			"question": {
				Desc:     "关于图片的问题或需要分析的内容。如果只需要一般性描述，可以留空",
				Type:     schema.String,
				Required: false,
			},
		}),
	}, nil
}

type VisionRequest struct {
	ImageURLs []string `json:"image_urls"`
	Question  string   `json:"question,omitempty"`
}

func (t *VisionTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args VisionRequest

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	// 验证参数
	if len(args.ImageURLs) == 0 {
		return "", fmt.Errorf("image_urls must contain at least one URL")
	}

	// 构建提示词
	prompt := "请仔细分析这张图片"
	if args.Question != "" {
		prompt = args.Question
	}

	// 批量处理图片
	results, err := t.processBatch(ctx, args.ImageURLs, prompt)
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
		fmt.Fprintf(&combined, "## Image %d Analysis\n\n%s", i+1, result)
	}

	return combined.String(), nil
}

func (t *VisionTool) processBatch(ctx context.Context, imageURLs []string, question string) ([]string, error) {
	results := make([]string, len(imageURLs))
	errors := make([]error, len(imageURLs))

	var wg sync.WaitGroup
	// 限制并发数为 3（视觉模型通常更耗资源）
	semaphore := make(chan struct{}, 3)

	for i, url := range imageURLs {
		wg.Add(1)
		go func(index int, imageURL string) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			// 构建多模态消息
			messages := []*schema.Message{
				{
					Role: schema.User,
					MultiContent: []schema.ChatMessagePart{
						{
							Type: schema.ChatMessagePartTypeText,
							Text: question,
						},
						{
							Type: schema.ChatMessagePartTypeImageURL,
							ImageURL: &schema.ChatMessageImageURL{
								URL: imageURL,
							},
						},
					},
				},
			}

			// 调用视觉模型
			result, err := t.visionModel.Generate(ctx, messages, model.WithMaxTokens(2000))
			if err != nil {
				errors[index] = fmt.Errorf("failed to analyze image %d: %w", index+1, err)
				return
			}

			results[index] = result.Content
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

func NewTool(ctx context.Context, visionModel types.ChatModel) (tool.InvokableTool, error) {
	if visionModel == nil {
		return nil, fmt.Errorf("vision model is required")
	}
	return &VisionTool{
		visionModel: visionModel,
	}, nil
}
