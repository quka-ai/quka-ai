package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	FUNCTION_NAME_CREATE_KNOWLEDGE    = "CreateKnowledge"
	FUNCTION_NAME_UPDATE_KNOWLEDGE    = "UpdateKnowledge"
	FUNCTION_NAME_LIST_USER_RESOURCES = "ListUserResources"
)

// KnowledgeLogicFunctions 知识逻辑层函数接口,用于依赖注入
type KnowledgeLogicFunctions struct {
	InsertContentAsyncWithSource func(spaceID, resource string, kind types.KnowledgeKind, content types.KnowledgeContent, contentType types.KnowledgeContentType, source types.KnowledgeSource, sourceRef string) (string, error)
	GetKnowledge                 func(spaceID, id string) (*types.Knowledge, error)
	Update                       func(spaceID, id string, args types.UpdateKnowledgeArgs) error
}

// ResourceLogicFunctions 资源逻辑层函数接口,用于依赖注入
type ResourceLogicFunctions struct {
	GetResource       func(spaceID, id string) (*types.Resource, error)
	ListUserResources func(userID string, page, pagesize uint64) ([]types.Resource, error)
}

// GetKnowledgeToolsWithLogic 通过依赖注入方式创建 knowledge tools,避免循环依赖
func GetKnowledgeToolsWithLogic(
	core *core.Core,
	spaceID, sessionID, userID string,
	knowledgeFuncs KnowledgeLogicFunctions,
	resourceFuncs ResourceLogicFunctions,
) []tool.InvokableTool {
	return []tool.InvokableTool{
		NewCreateKnowledgeTool(core, spaceID, sessionID, userID, knowledgeFuncs, resourceFuncs),
		NewUpdateKnowledgeTool(core, spaceID, userID, knowledgeFuncs, resourceFuncs),
		NewListUserResourcesTool(core, userID, resourceFuncs),
	}
}

// CreateKnowledgeTool 创建知识工具
type CreateKnowledgeTool struct {
	core           *core.Core
	spaceID        string
	sessionID      string
	userID         string
	knowledgeFuncs KnowledgeLogicFunctions
	resourceFuncs  ResourceLogicFunctions
}

func NewCreateKnowledgeTool(
	core *core.Core,
	spaceID, sessionID, userID string,
	knowledgeFuncs KnowledgeLogicFunctions,
	resourceFuncs ResourceLogicFunctions,
) *CreateKnowledgeTool {
	return &CreateKnowledgeTool{
		core:           core,
		spaceID:        spaceID,
		sessionID:      sessionID,
		userID:         userID,
		knowledgeFuncs: knowledgeFuncs,
		resourceFuncs:  resourceFuncs,
	}
}

var _ tool.InvokableTool = (*CreateKnowledgeTool)(nil)

// Info 实现 BaseTool 接口
func (t *CreateKnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"resource": {
			Type:     schema.String,
			Desc:     "资源分类ID,不指定则使用默认分类 'knowledge'。建议先使用 ListUserResources 查看可用选项",
			Required: false,
		},
		"title": {
			Type:     schema.String,
			Desc:     "知识标题（可选）。如果不提供，系统会根据聊天内容自动生成",
			Required: false,
		},
	}

	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_CREATE_KNOWLEDGE,
		Desc:        "基于当前对话历史创建知识(记忆)条目。系统会自动总结当前会话的关键信息，生成结构化的知识内容。Resource 是知识的分类标识,如果不指定,将保存到默认分类(knowledge)。",
		ParamsOneOf: paramsOneOf,
	}, nil
}

type CreateKnowledgeParams struct {
	Resource string `json:"resource"`
	Title    string `json:"title"`
}

// InvokableRun 实现 InvokableTool 接口
func (t *CreateKnowledgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 1. 解析参数
	var params CreateKnowledgeParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("Invalid parameters: %w", err)
	}

	// 2. 并发控制 - 使用分布式信号量限制同时生成总结的用户数
	semaphore := t.core.Semaphore().KnowledgeSummary()
	if !semaphore.TryAcquire() {
		return "", fmt.Errorf("System is busy generating knowledge summaries. Please try again in a moment.")
	}
	defer semaphore.Release()

	// 3. 获取当前会话的聊天历史（从最近的总结往后获取）
	chatHistory, err := t.getChatHistoryFromLastSummary(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get chat history: %w", err)
	}

	if len(chatHistory) == 0 {
		return "", fmt.Errorf("No chat history available to create knowledge")
	}

	// 4. 使用 AI 生成知识总结
	knowledgeContent, generatedTitle, err := t.generateKnowledgeSummary(ctx, chatHistory, params.Title)
	if err != nil {
		return "", fmt.Errorf("failed to generate knowledge summary: %w", err)
	}

	// 5. 如果用户没有提供标题，使用生成的标题
	if params.Title == "" {
		params.Title = generatedTitle
	}

	// 6. 匹配 resource（保持现有逻辑）
	resource := params.Resource
	if resource == "" || strings.ToLower(resource) == types.DEFAULT_RESOURCE {
		resource = types.DEFAULT_RESOURCE
	} else {
		matchedResourceID, err := t.matchResourceByDescription(ctx, resource)
		if err != nil {
			return fmt.Sprintf("Failed to match resource: %s", err.Error()), nil
		}
		if matchedResourceID == "" {
			return fmt.Sprintf("Could not find a matching resource for '%s'. Use ListUserResources to see available resources.", resource), nil
		}
		resource = matchedResourceID

		if resource != types.DEFAULT_RESOURCE {
			// 验证匹配的 resource 确实存在
			res, err := t.resourceFuncs.GetResource(t.spaceID, resource)
			if err != nil || res == nil {
				return fmt.Sprintf("Matched resource '%s' not found. Please try again.", resource), nil
			}
		}
	}

	// 7. 创建 knowledge
	knowledgeID, err := t.knowledgeFuncs.InsertContentAsyncWithSource(
		t.spaceID,
		resource,
		types.KNOWLEDGE_KIND_TEXT,
		types.KnowledgeContent(knowledgeContent),
		types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
		types.KNOWLEDGE_SOURCE_CHAT,
		t.sessionID,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create knowledge: %w", err)
	}

	// 8. 记录日志
	slog.Info("CreateKnowledgeTool: knowledge created from chat history",
		slog.String("user_id", t.userID),
		slog.String("space_id", t.spaceID),
		slog.String("knowledge_id", knowledgeID),
		slog.String("resource", resource),
		slog.String("title", params.Title),
		slog.Int("content_length", len(knowledgeContent)),
		slog.Int("history_message_count", len(chatHistory)),
	)

	// 9. 返回结果
	return fmt.Sprintf("Knowledge created successfully from conversation history!\nID: %s\nTitle: %s\nResource: %s\nThe knowledge is being processed (summarization and embedding) in background.",
		knowledgeID, params.Title, resource), nil
}

// matchResourceByDescription 使用 AI 模型将用户的自然语言描述匹配到实际的 resource ID
func (t *CreateKnowledgeTool) matchResourceByDescription(ctx context.Context, userDescription string) (string, error) {
	// 1. 获取所有可用的 resources
	resources, err := t.resourceFuncs.ListUserResources(t.userID, 0, 0)
	if err != nil {
		return "", fmt.Errorf("failed to list resources: %w", err)
	}

	if len(resources) == 0 {
		// 没有自定义资源,返回默认值
		return types.DEFAULT_RESOURCE, nil
	}

	// 2. 首先尝试精确匹配或部分匹配(不区分大小写)
	userDescLower := strings.ToLower(strings.TrimSpace(userDescription))
	for _, r := range resources {
		// 精确匹配 ID
		if strings.ToLower(r.ID) == userDescLower {
			return r.ID, nil
		}
		// 精确匹配 Title
		if strings.ToLower(r.Title) == userDescLower {
			return r.ID, nil
		}
	}

	// 3. 如果没有精确匹配,使用 AI 模型进行语义匹配
	matchedID, err := t.semanticMatchResource(ctx, userDescription, resources)
	if err != nil {
		slog.Warn("Semantic resource matching failed, falling back to partial match",
			slog.String("error", err.Error()),
			slog.String("user_description", userDescription))

		// AI 匹配失败,尝试部分匹配
		return t.fallbackPartialMatch(userDescription, resources), nil
	}

	return matchedID, nil
}

// semanticMatchResource 使用 AI 模型进行语义匹配
func (t *CreateKnowledgeTool) semanticMatchResource(ctx context.Context, userDescription string, resources []types.Resource) (string, error) {
	// 构建资源列表描述
	var resourceList strings.Builder
	resourceList.WriteString("Available resources:\n")
	for _, r := range resources {
		desc := r.Description
		if desc == "" {
			desc = "No description"
		}
		resourceList.WriteString(fmt.Sprintf("- ID: %s, Title: %s, Description: %s\n", r.ID, r.Title, desc))
	}

	// 构建 prompt
	systemPrompt := `You are a resource matching assistant. Your task is to match a user's natural language description to the most appropriate resource ID from the available list.

Instructions:
1. Analyze the user's description and match it to the most appropriate resource based on the ID, title, and description
2. Consider semantic similarity, not just keyword matching
3. Return ONLY the resource ID (nothing else, no explanation)
4. If no good match is found, return "knowledge" as the default`

	userPrompt := fmt.Sprintf(`User's description: "%s"

%s

Resource ID:`, userDescription, resourceList.String())

	// 获取 AI 模型并调用
	chatModel := t.core.Srv().AI().GetChatAI(false)
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: userPrompt,
		},
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("AI service error: %w", err)
	}

	// 记录 AI token 使用量
	go safe.Run(func() {
		t.recordTokenUsage(ctx, chatModel, resp, "resource_matching")
	})

	// 解析响应
	matchedID := strings.TrimSpace(resp.Content)
	matchedID = strings.Trim(matchedID, "\"'`") // 去除可能的引号

	// 验证返回的 ID 是否在资源列表中
	for _, r := range resources {
		if r.ID == matchedID {
			slog.Info("AI successfully matched resource",
				slog.String("user_description", userDescription),
				slog.String("matched_id", matchedID))
			return matchedID, nil
		}
	}

	// 如果 AI 返回的不在列表中,尝试部分匹配
	if matchedID == "knowledge" || matchedID == "" {
		return types.DEFAULT_RESOURCE, nil
	}

	return "", fmt.Errorf("AI returned invalid resource ID: %s", matchedID)
}

// recordTokenUsage 记录 AI token 使用量到数据库
func (t *CreateKnowledgeTool) recordTokenUsage(ctx context.Context, chatModel types.ChatModel, resp *schema.Message, subType string) {
	if resp.ResponseMeta == nil || resp.ResponseMeta.Usage == nil {
		slog.Warn("No usage metadata in AI response for token recording")
		return
	}

	usage := resp.ResponseMeta.Usage
	modelName := chatModel.Config().ModelName

	if err := t.core.Store().AITokenUsageStore().Create(ctx, types.AITokenUsage{
		SpaceID:     t.spaceID,
		UserID:      t.userID,
		Type:        types.USAGE_TYPE_KNOWLEDGE,
		SubType:     subType,
		ObjectID:    "", // 此时 knowledge 还未创建,留空
		Model:       modelName,
		UsagePrompt: usage.PromptTokens,
		UsageOutput: usage.CompletionTokens,
		CreatedAt:   time.Now().Unix(),
	}); err != nil {
		slog.Error("Failed to record token usage for resource matching",
			slog.String("user_id", t.userID),
			slog.String("space_id", t.spaceID),
			slog.String("model", modelName),
			slog.String("sub_type", subType),
			slog.String("error", err.Error()))
		// 不返回错误,因为这不应该影响主流程
	} else {
		slog.Debug("Token usage recorded",
			slog.String("user_id", t.userID),
			slog.String("model", modelName),
			slog.String("sub_type", subType),
			slog.Int("prompt_tokens", usage.PromptTokens),
			slog.Int("completion_tokens", usage.CompletionTokens),
			slog.Int("total_tokens", usage.TotalTokens))
	}
}

// fallbackPartialMatch 当 AI 匹配失败时,使用简单的部分字符串匹配
func (t *CreateKnowledgeTool) fallbackPartialMatch(userDescription string, resources []types.Resource) string {
	userDescLower := strings.ToLower(userDescription)

	// 尝试在 Title 中查找部分匹配
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.Title), userDescLower) {
			slog.Info("Fallback partial match found in title",
				slog.String("user_description", userDescription),
				slog.String("matched_id", r.ID))
			return r.ID
		}
	}

	// 尝试在 Description 中查找部分匹配
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.Description), userDescLower) {
			slog.Info("Fallback partial match found in description",
				slog.String("user_description", userDescription),
				slog.String("matched_id", r.ID))
			return r.ID
		}
	}

	// 如果都没有匹配,返回空字符串,让调用者处理
	return ""
}

// UpdateKnowledgeTool 更新知识工具
type UpdateKnowledgeTool struct {
	core           *core.Core
	spaceID        string
	userID         string
	knowledgeFuncs KnowledgeLogicFunctions
	resourceFuncs  ResourceLogicFunctions
}

func NewUpdateKnowledgeTool(
	core *core.Core,
	spaceID, userID string,
	knowledgeFuncs KnowledgeLogicFunctions,
	resourceFuncs ResourceLogicFunctions,
) *UpdateKnowledgeTool {
	return &UpdateKnowledgeTool{
		core:           core,
		spaceID:        spaceID,
		userID:         userID,
		knowledgeFuncs: knowledgeFuncs,
		resourceFuncs:  resourceFuncs,
	}
}

var _ tool.InvokableTool = (*UpdateKnowledgeTool)(nil)

// Info 实现 BaseTool 接口
func (t *UpdateKnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{
		"id": {
			Type:     schema.String,
			Desc:     "要更新的 knowledge ID",
			Required: true,
		},
		"content": {
			Type:     schema.String,
			Desc:     "新内容(markdown格式),如果提供,知识将被重新处理(summarization + embedding)",
			Required: false,
		},
		"resource": {
			Type:     schema.String,
			Desc:     "移动到新的 resource 分类,建议先使用 ListUserResources 确认目标 resource 存在",
			Required: false,
		},
		"title": {
			Type:     schema.String,
			Desc:     "新标题",
			Required: false,
		},
	}

	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_UPDATE_KNOWLEDGE,
		Desc:        "更新已存在的知识条目。只需提供要更新的字段,未提供的字段保持不变。更新 content 会触发异步处理,更新 resource 会移动知识到新的分类。只能更新属于当前用户的 knowledge。",
		ParamsOneOf: paramsOneOf,
	}, nil
}

type UpdateKnowledgeParams struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Resource string `json:"resource"`
	Title    string `json:"title"`
}

// InvokableRun 实现 InvokableTool 接口
func (t *UpdateKnowledgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 1. 解析参数
	var params UpdateKnowledgeParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "Invalid parameters. Please check your input format.", nil
	}

	if params.ID == "" {
		return "Error: knowledge ID is required", nil
	}

	// 2. 验证 knowledge 存在且属于当前用户
	existing, err := t.knowledgeFuncs.GetKnowledge(t.spaceID, params.ID)
	if err != nil {
		return fmt.Sprintf("Knowledge not found: %s", params.ID), nil
	}

	if existing.UserID != t.userID {
		return "Error: You don't have permission to update this knowledge", nil
	}

	// 3. 构建更新参数
	updateArgs := types.UpdateKnowledgeArgs{}
	updatedFields := []string{}

	if params.Title != "" {
		updateArgs.Title = params.Title
		updatedFields = append(updatedFields, "title")
	}

	if params.Content != "" {
		if len(params.Content) > 100000 {
			return "Error: content is too long (max 100KB)", nil
		}

		updateArgs.Content = types.KnowledgeContent(params.Content)
		updateArgs.ContentType = types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN
		updatedFields = append(updatedFields, "content")
	}

	if params.Resource != "" && params.Resource != existing.Resource {
		// 验证目标 resource 存在
		if params.Resource != types.DEFAULT_RESOURCE {
			targetResource, err := t.resourceFuncs.GetResource(t.spaceID, params.Resource)
			if err != nil || targetResource == nil {
				return fmt.Sprintf("Target resource '%s' not found. Use ListUserResources to see available resources.",
					params.Resource), nil
			}
		}
		updateArgs.Resource = params.Resource
		updatedFields = append(updatedFields, "resource")
	}

	if len(updatedFields) == 0 {
		return "No fields to update. Please provide at least one field (content, resource, title, or tags).", nil
	}

	// 4. 执行更新
	err = t.knowledgeFuncs.Update(t.spaceID, params.ID, updateArgs)
	if err != nil {
		return "", fmt.Errorf("failed to update knowledge: %w", err)
	}

	// 5. 记录日志
	slog.Info("UpdateKnowledgeTool: knowledge updated",
		slog.String("user_id", t.userID),
		slog.String("space_id", t.spaceID),
		slog.String("knowledge_id", params.ID),
		slog.String("updated_fields", strings.Join(updatedFields, ", ")),
	)

	// 6. 返回结果
	status := "updated"
	if params.Content != "" {
		status = "updated and re-processing"
	}

	return fmt.Sprintf("Knowledge %s successfully!\nID: %s\nUpdated fields: %s",
		status, params.ID, strings.Join(updatedFields, ", ")), nil
}

// ListUserResourcesTool 列出用户资源工具
type ListUserResourcesTool struct {
	core          *core.Core
	userID        string
	resourceFuncs ResourceLogicFunctions
}

func NewListUserResourcesTool(
	core *core.Core,
	userID string,
	resourceFuncs ResourceLogicFunctions,
) *ListUserResourcesTool {
	return &ListUserResourcesTool{
		core:          core,
		userID:        userID,
		resourceFuncs: resourceFuncs,
	}
}

var _ tool.InvokableTool = (*ListUserResourcesTool)(nil)

// Info 实现 BaseTool 接口
func (t *ListUserResourcesTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{}

	paramsOneOf := schema.NewParamsOneOfByParams(params)

	return &schema.ToolInfo{
		Name:        FUNCTION_NAME_LIST_USER_RESOURCES,
		Desc:        "列出用户可以使用的所有 resource(知识分类)。Resource 是用于组织和管理知识的分类标识,每个 resource 可以有标题、描述和周期(知识过期时间)。使用场景:在创建或更新 knowledge 前,查看可用的分类选项。",
		ParamsOneOf: paramsOneOf,
	}, nil
}

// InvokableRun 实现 InvokableTool 接口
func (t *ListUserResourcesTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 1. 调用 ResourceLogic 获取用户的 resources
	resources, err := t.resourceFuncs.ListUserResources(t.userID, 0, 0) // 不分页,返回所有
	if err != nil {
		return "", fmt.Errorf("failed to list user resources: %w", err)
	}

	if len(resources) == 0 {
		return "You don't have any custom resources yet. Knowledge will be saved to the default resource 'knowledge'.", nil
	}

	// 2. 格式化输出
	sb := strings.Builder{}
	sb.WriteString("Available Resources:\n\n")
	sb.WriteString("| ID | Title | Description | Lifecycle |\n")
	sb.WriteString("| --- | --- | --- | --- |\n")

	for _, r := range resources {
		lifecycle := "Permanent"
		if r.Cycle > 0 {
			lifecycle = fmt.Sprintf("%d days", r.Cycle)
		}

		desc := r.Description
		if desc == "" {
			desc = "-"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			r.ID, r.Title, desc, lifecycle))
	}

	sb.WriteString("\nUsage:\n")
	sb.WriteString("- Use the 'ID' column value when creating or updating knowledge\n")
	sb.WriteString("- If no resource is specified, knowledge will be saved to 'knowledge'\n")

	return sb.String(), nil
}

// getChatHistoryFromLastSummary 获取从最近一次总结到当前的聊天历史
// 参考 ai.go 中的 GenChatSessionContextAndSummaryIfExceedsTokenLimit 方法
func (t *CreateKnowledgeTool) getChatHistoryFromLastSummary(ctx context.Context) ([]*types.MessageContext, error) {
	// 1. 获取最近的总结（如果存在）
	summary, err := t.core.Store().ChatSummaryStore().GetChatSessionLatestSummary(ctx, t.sessionID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get latest summary: %w", err)
	}

	var summarySequence int64 = 0
	if summary != nil {
		summarySequence = summary.Sequence
	}

	// 2. 获取比 summary sequence 更大的聊天内容
	msgList, err := t.core.Store().ChatMessageStore().ListSessionMessage(
		ctx,
		t.spaceID,
		t.sessionID,
		summarySequence, // 从总结之后开始获取
		types.NO_PAGINATION,
		types.NO_PAGINATION,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list session messages: %w", err)
	}

	// 3. 按 sequence 排序
	sort.Slice(msgList, func(i, j int) bool {
		return msgList[i].Sequence < msgList[j].Sequence
	})

	fmt.Println("len message", len(msgList))
	// 4. 转换为 MessageContext 并解密
	var messageContexts []*types.MessageContext
	for _, msg := range msgList {
		// 解密消息（如果加密）
		if msg.IsEncrypt == types.MESSAGE_IS_ENCRYPT {
			decrypted, err := t.core.DecryptData([]byte(msg.Message))
			if err != nil {
				slog.Error("Failed to decrypt message for knowledge creation",
					slog.String("message_id", msg.ID),
					slog.String("error", err.Error()))
				continue
			}
			msg.Message = string(decrypted)
		}

		// 只包含完成的消息
		if msg.Complete != types.MESSAGE_PROGRESS_COMPLETE {
			fmt.Println("skip one")
			continue
		}

		// 跳过错误消息
		if isErrorMessage(msg.Message) {
			fmt.Println("skip one one")
			continue
		}

		messageContexts = append(messageContexts, &types.MessageContext{
			Role:    msg.Role,
			Content: msg.Message,
		})
	}

	return messageContexts, nil
}

// isErrorMessage 检查消息是否为错误消息（参考 ai.go）
func isErrorMessage(msg string) bool {
	msg = strings.TrimSpace(msg)
	if strings.HasPrefix(msg, "Sorry，") || strings.HasPrefix(msg, "抱歉，") || msg == "" {
		return true
	}
	return false
}

// generateKnowledgeSummary 使用 AI 从聊天历史生成知识总结
func (t *CreateKnowledgeTool) generateKnowledgeSummary(ctx context.Context, chatHistory []*types.MessageContext, userProvidedTitle string) (content string, title string, err error) {
	// 1. 验证输入
	if len(chatHistory) == 0 {
		return "", "", fmt.Errorf("chat history is empty")
	}

	// 2. 构建聊天历史的文本表示
	var historyBuilder strings.Builder
	for _, msg := range chatHistory {
		role := "User"
		if msg.Role == types.USER_ROLE_ASSISTANT {
			role = "Assistant"
		} else if msg.Role == types.USER_ROLE_SYSTEM {
			role = "System"
		} else if msg.Role == types.USER_ROLE_TOOL {
			role = "Tool"
		}

		historyBuilder.WriteString(fmt.Sprintf("%s: %s\n\n", role, msg.Content))
	}

	// 3. 构建系统 prompt
	systemPrompt := `你是一个专业的知识总结助手。你的任务是从对话历史中提取关键信息，生成结构化的知识条目。

要求：
1. 使用标准的 Markdown 格式
2. 提取对话中的核心概念、重要信息和结论
3. 组织成清晰的结构（使用标题、列表、代码块等）
4. 保留重要的技术细节和上下文
5. 去除无关的对话内容（如问候语、确认等）
6. 如果对话包含代码，请使用代码块格式化
7. 如果对话包含步骤，使用有序或无序列表
8. 总结应该独立可读，不依赖原始对话

输出格式：
- 如果用户已提供标题，直接输出内容（JSON 格式：{"content": "...", "title": ""}）
- 如果用户未提供标题，同时生成标题和内容（JSON 格式：{"content": "...", "title": "..."}）
- 标题应简洁明了，反映知识的核心主题（不超过50字符）
- 内容必须是有效的 Markdown 格式`

	// 4. 构建用户 prompt
	titleInstruction := "请为这段对话生成一个简洁的标题和结构化的总结。"
	if userProvidedTitle != "" {
		titleInstruction = fmt.Sprintf("用户已提供标题：'%s'。请生成结构化的总结内容。", userProvidedTitle)
	}

	userPrompt := fmt.Sprintf(`%s

对话历史：
%s

请以 JSON 格式返回结果：
{"content": "markdown格式的总结内容", "title": "标题（如果用户已提供则留空）"}`,
		titleInstruction, historyBuilder.String())

	// 5. 调用 AI 模型
	chatModel := t.core.Srv().AI().GetChatAI(false)
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: userPrompt,
		},
	}

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", "", fmt.Errorf("AI service error: %w", err)
	}

	// 6. 记录 token 使用量
	go safe.Run(func() {
		t.recordTokenUsage(ctx, chatModel, resp, "knowledge_summarization")
	})

	// 7. 解析响应
	var result struct {
		Content string `json:"content"`
		Title   string `json:"title"`
	}

	// 尝试清理可能的 markdown 代码块标记
	cleanedContent := strings.TrimSpace(resp.Content)
	cleanedContent = strings.TrimPrefix(cleanedContent, "```json")
	cleanedContent = strings.TrimPrefix(cleanedContent, "```")
	cleanedContent = strings.TrimSuffix(cleanedContent, "```")
	cleanedContent = strings.TrimSpace(cleanedContent)

	if err := json.Unmarshal([]byte(cleanedContent), &result); err != nil {
		return "", "", fmt.Errorf("failed to parse AI response: %w", err)
	}

	// 8. 验证内容
	if result.Content == "" {
		return "", "", fmt.Errorf("AI generated empty content")
	}

	// 9. 如果用户提供了标题，使用用户的标题；否则使用生成的标题
	finalTitle := userProvidedTitle
	if finalTitle == "" {
		finalTitle = result.Title
	}
	if finalTitle == "" {
		finalTitle = "Untitled Knowledge"
	}

	slog.Info("Knowledge summary generated",
		slog.String("user_id", t.userID),
		slog.String("session_id", t.sessionID),
		slog.String("generated_title", result.Title),
		slog.String("final_title", finalTitle),
		slog.Int("content_length", len(result.Content)),
	)

	return result.Content, finalTitle, nil
}
