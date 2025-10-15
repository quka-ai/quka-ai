package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/quka-ai/quka-ai/app/core"
	v1 "github.com/quka-ai/quka-ai/app/logic/v1"
	"github.com/quka-ai/quka-ai/pkg/mcp/auth"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils/editorjs"
	"github.com/samber/lo"
)

// CreateKnowledgeInput 创建知识的输入参数
type CreateKnowledgeInput struct {
	Content     string   `json:"content" jsonschema:"The content of the knowledge (markdown or plain text)"`
	ContentType string   `json:"content_type,omitempty" jsonschema:"Content format type (markdown or blocks)"`
	Resource    string   `json:"resource,omitempty" jsonschema:"Resource identifier, e.g., 'knowledge'"`
	Kind        string   `json:"kind,omitempty" jsonschema:"Type of knowledge (text, image, video, or url)"`
	Title       string   `json:"title,omitempty" jsonschema:"Optional title for the knowledge"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Optional tags for categorization"`
}

// CreateKnowledgeOutput 创建知识的输出结果
type CreateKnowledgeOutput struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CreateKnowledgeHandler 创建知识的处理器
type CreateKnowledgeHandler struct {
	core *core.Core
}

// NewCreateKnowledgeHandler 创建新的知识创建处理器
func NewCreateKnowledgeHandler(core *core.Core) *CreateKnowledgeHandler {
	return &CreateKnowledgeHandler{core: core}
}

// Handle 处理创建知识请求
func (h *CreateKnowledgeHandler) Handle(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args CreateKnowledgeInput,
) (*mcp.CallToolResult, CreateKnowledgeOutput, error) {
	// 从 context 获取认证信息（由 auth middleware 注入）
	userCtx, ok := auth.GetUserContext(ctx)
	if !ok {
		return nil, CreateKnowledgeOutput{}, fmt.Errorf("user context not found")
	}

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
		userCtx.Field("space_id"),
		lo.If(args.Resource != "", args.Resource).Else(userCtx.Field("resource")),
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
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Knowledge created: " + id},
		},
	}, output, nil
}

// GetKnowledgeInput 获取知识的输入参数
type GetKnowledgeInput struct {
	ID string `json:"id" jsonschema:"The unique identifier of the knowledge to retrieve"`
}

// GetKnowledgeOutput 获取知识的输出
type GetKnowledgeOutput struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	ContentType string   `json:"content_type"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
}

// GetKnowledgeHandler 处理获取知识请求
type GetKnowledgeHandler struct {
	core *core.Core
}

// NewGetKnowledgeHandler 创建新的获取知识处理器
func NewGetKnowledgeHandler(core *core.Core) *GetKnowledgeHandler {
	return &GetKnowledgeHandler{core: core}
}

// Handle 处理获取知识请求
func (h *GetKnowledgeHandler) Handle(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args GetKnowledgeInput,
) (*mcp.CallToolResult, GetKnowledgeOutput, error) {
	// 从 context 获取认证信息
	userCtx, ok := auth.GetUserContext(ctx)
	if !ok {
		return nil, GetKnowledgeOutput{}, fmt.Errorf("user context not found")
	}

	// 验证 ID
	if args.ID == "" {
		return nil, GetKnowledgeOutput{}, fmt.Errorf("knowledge ID is required")
	}

	// 调用 KnowledgeLogic 获取知识
	logic := v1.NewKnowledgeLogic(ctx, h.core)
	knowledge, err := logic.GetKnowledge(userCtx.Field("space_id"), args.ID)
	if err != nil {
		return nil, GetKnowledgeOutput{}, fmt.Errorf("failed to get knowledge: %w", err)
	}

	if knowledge == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "knowledge not found: " + args.ID},
			},
		}, GetKnowledgeOutput{}, nil
	}

	var contentStr string
	if knowledge.ContentType == types.KNOWLEDGE_CONTENT_TYPE_BLOCKS {
		blocks, err := editorjs.ParseRawToBlocks(json.RawMessage(knowledge.Content))
		if err != nil {
			slog.Error("Failed to parse editor blocks", slog.String("knowledge_id", knowledge.ID), slog.String("error", err.Error()))
		}

		if len(blocks.Blocks) > 6 {
			blocks.Blocks = blocks.Blocks[:6]
		}

		knowledge.ContentType = types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN
		contentStr, err = editorjs.ConvertEditorJSBlocksToMarkdown(blocks.Blocks)
		if err != nil {
			slog.Error("Failed to convert editor blocks to markdown", slog.String("knowledge_id", knowledge.ID), slog.String("error", err.Error()))
		}
	}

	// 构建输出
	output := GetKnowledgeOutput{
		ID:          knowledge.ID,
		Content:     contentStr,
		ContentType: string(knowledge.ContentType),
		Kind:        knowledge.Kind.String(),
		CreatedAt:   knowledge.CreatedAt,
		UpdatedAt:   knowledge.UpdatedAt,
	}

	// 格式化内容用于显示
	displayContent := fmt.Sprintf("# Knowledge: %s\n\n**ID**: %s\n**Type**: %s\n**Created**: %s\n\n---\n\n%s",
		args.ID,
		knowledge.ID,
		knowledge.Kind.String(),
		formatTimestamp(knowledge.CreatedAt),
		contentStr,
	)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: displayContent},
		},
	}, output, nil
}

// formatTimestamp 格式化时间戳
func formatTimestamp(ts int64) string {
	if ts == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%d", ts)
}

// RegisterCreateKnowledgeTool 注册 create_knowledge 工具
func RegisterCreateKnowledgeTool(server *mcp.Server, core *core.Core) {
	handler := NewCreateKnowledgeHandler(core)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_knowledge",
		Description: "Create a new knowledge entry in QukaAI. The knowledge will be processed asynchronously (summarization and embedding) in the background.",
	}, handler.Handle)
}

// RegisterGetKnowledgeTool 注册 get_knowledge 工具
func RegisterGetKnowledgeTool(server *mcp.Server, core *core.Core) {
	handler := NewGetKnowledgeHandler(core)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_knowledge",
		Description: "Retrieve a knowledge entry by its ID. Returns the full content and metadata of the knowledge.",
	}, handler.Handle)
}
