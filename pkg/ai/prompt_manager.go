package ai

import (
	"strconv"
	"strings"

	"github.com/quka-ai/quka-ai/pkg/types"
)

// PromptTemplate 代表一个完整的 prompt 模板
// 采用三段式结构：Header（头部）+ Body（中间）+ Append（尾部）
// 只有 Body 部分允许业务逻辑修改
type PromptTemplate struct {
	Header string            // 头部：项目信息、时间、基本约束（不可修改）
	Body   string            // 中间：业务逻辑自定义部分（可修改）
	Append string            // 尾部：系统规范、语法说明（不可修改）
	Lang   string            // 语言：cn/en
	Vars   map[string]string // 变量映射表
}

// Build 构建完整的 prompt
// 按照 Header + Body + Append 的顺序拼接，然后替换所有变量
func (pt *PromptTemplate) Build() string {
	prompt := pt.Header + "\n\n" + pt.Body + "\n\n" + pt.Append

	// 替换所有变量
	for k, v := range pt.Vars {
		prompt = strings.ReplaceAll(prompt, k, v)
	}

	return prompt
}

// SetBody 设置中间部分（业务逻辑唯一可修改的地方）
func (pt *PromptTemplate) SetBody(body string) {
	pt.Body = body
}

// AppendBody 追加内容到中间部分
func (pt *PromptTemplate) AppendBody(content string) {
	if pt.Body == "" {
		pt.Body = content
	} else {
		pt.Body += "\n\n" + content
	}
}

// SetVar 设置变量
func (pt *PromptTemplate) SetVar(key, value string) {
	if pt.Vars == nil {
		pt.Vars = make(map[string]string)
	}
	pt.Vars[key] = value
}

// PromptConfig 配置结构（与 app/core/config.go 对应）
type PromptConfig struct {
	Header       string // 全局头部
	ChatSummary  string // 聊天总结
	EnhanceQuery string // 查询增强
	SessionName  string // 会话命名
}

// DefaultPrompt 系统默认 prompt
type DefaultPrompt struct {
	HeaderCN string
	HeaderEN string
	AppendCN string
	AppendEN string
}

// PromptManager 管理所有 prompt 模板
type PromptManager struct {
	config         *PromptConfig             // 从 core.Config 获取
	defaultPrompts map[string]*DefaultPrompt // 系统默认 prompt
	lang           string                    // 默认语言
}

// NewPromptManager 创建 prompt 管理器
func NewPromptManager(config *PromptConfig, lang string) *PromptManager {
	if lang == "" {
		lang = MODEL_BASE_LANGUAGE_CN
	}

	pm := &PromptManager{
		config:         config,
		lang:           lang,
		defaultPrompts: make(map[string]*DefaultPrompt),
	}

	// 初始化系统默认 prompt
	pm.initDefaultPrompts()

	return pm
}

// initDefaultPrompts 初始化系统默认 prompt
func (pm *PromptManager) initDefaultPrompts() {
	// Chat 场景
	pm.defaultPrompts["chat"] = &DefaultPrompt{
		HeaderCN: PROMPT_HEADER_CHAT_CN,
		HeaderEN: PROMPT_HEADER_CHAT_EN,
		AppendCN: APPEND_PROMPT_CN,
		AppendEN: APPEND_PROMPT_EN,
	}

	// RAG 场景
	pm.defaultPrompts["rag"] = &DefaultPrompt{
		HeaderCN: PROMPT_HEADER_RAG_CN,
		HeaderEN: PROMPT_HEADER_RAG_EN,
		AppendCN: APPEND_PROMPT_CN,
		AppendEN: APPEND_PROMPT_EN,
	}

	// Summary 场景
	pm.defaultPrompts["summary"] = &DefaultPrompt{
		HeaderCN: PROMPT_HEADER_SUMMARY_CN,
		HeaderEN: PROMPT_HEADER_SUMMARY_EN,
		AppendCN: APPEND_PROMPT_CN,
		AppendEN: APPEND_PROMPT_EN,
	}

	// EnhanceQuery 场景
	pm.defaultPrompts["enhance_query"] = &DefaultPrompt{
		HeaderCN: PROMPT_HEADER_ENHANCE_QUERY_CN,
		HeaderEN: PROMPT_HEADER_ENHANCE_QUERY_EN,
		AppendCN: APPEND_PROMPT_CN,
		AppendEN: APPEND_PROMPT_EN,
	}

	// Butler 场景
	pm.defaultPrompts["butler"] = &DefaultPrompt{
		HeaderCN: PROMPT_HEADER_BUTLER_CN,
		HeaderEN: PROMPT_HEADER_BUTLER_EN,
		AppendCN: APPEND_PROMPT_CN,
		AppendEN: APPEND_PROMPT_EN,
	}

	// Journal 场景
	pm.defaultPrompts["journal"] = &DefaultPrompt{
		HeaderCN: PROMPT_HEADER_JOURNAL_CN,
		HeaderEN: PROMPT_HEADER_JOURNAL_EN,
		AppendCN: APPEND_PROMPT_CN,
		AppendEN: APPEND_PROMPT_EN,
	}

	// rag_tool_response 不需要在 defaultPrompts 中注册，通过专门方法获取
	// pm.defaultPrompts["rag_tool_response"] = &DefaultPrompt{}
}

// NewTemplate 创建一个新的 prompt 模板
// scenario: "chat", "rag", "summary", "enhance_query", "butler", "journal" 等
func (pm *PromptManager) NewTemplate(scenario string, lang string) *PromptTemplate {
	if lang == "" {
		lang = pm.lang
	}

	template := &PromptTemplate{
		Lang: lang,
		Vars: make(map[string]string),
	}

	// 设置头部（配置 → 系统默认）
	template.Header = pm.getHeader(scenario, lang)

	// 设置尾部（始终使用系统默认）
	template.Append = pm.getAppend(lang)

	// 设置通用变量
	pm.setCommonVars(template)

	return template
}

// getHeader 获取头部 prompt（配置优先）
func (pm *PromptManager) getHeader(scenario, lang string) string {
	// 优先使用配置中的头部
	if pm.config != nil && pm.config.Header != "" {
		return pm.config.Header
	}

	// 降级到系统默认
	defaultPrompt, ok := pm.defaultPrompts[scenario]
	if !ok {
		defaultPrompt = pm.defaultPrompts["chat"]
	}

	if lang == MODEL_BASE_LANGUAGE_EN {
		return defaultPrompt.HeaderEN
	}
	return defaultPrompt.HeaderCN
}

// getAppend 获取尾部 prompt（始终系统默认）
func (pm *PromptManager) getAppend(lang string) string {
	defaultPrompt := pm.defaultPrompts["chat"]

	if lang == MODEL_BASE_LANGUAGE_EN {
		return defaultPrompt.AppendEN
	}
	return defaultPrompt.AppendCN
}

// setCommonVars 设置通用变量
func (pm *PromptManager) setCommonVars(template *PromptTemplate) {
	// 设置站点信息
	template.SetVar(PROMPT_VAR_SITE_TITLE, SITE_TITLE)

	// 设置时间信息
	if template.Lang == MODEL_BASE_LANGUAGE_EN {
		template.SetVar(PROMPT_VAR_TIME_RANGE, GenerateTimeListAtNowEN())
		template.SetVar(PROMPT_VAR_SYMBOL, CurrentSymbols)
	} else {
		template.SetVar(PROMPT_VAR_TIME_RANGE, GenerateTimeListAtNowCN())
		template.SetVar(PROMPT_VAR_SYMBOL, CurrentSymbols)
	}
}

// GetChatTemplate 获取聊天场景的模板
func (pm *PromptManager) GetChatTemplate(lang string, space *types.Space) *PromptTemplate {
	template := pm.NewTemplate("chat", lang)

	// 设置中间部分
	body := ""

	// 1. Space 自定义 BasePrompt
	if space != nil && space.BasePrompt != "" {
		template.Header = space.BasePrompt + "\n\n"
	}

	if space != nil && space.ChatPrompt != "" {
		body += "\n\n" + space.ChatPrompt
	}

	// 2. 基础生成 Prompt
	if lang == MODEL_BASE_LANGUAGE_EN {
		body += BASE_GENERATE_PROMPT_EN
	} else {
		body += BASE_GENERATE_PROMPT_CN
	}

	template.SetBody(body)
	return template
}

// GetRAGTemplate 获取 RAG 场景的模板
func (pm *PromptManager) GetRAGTemplate(lang string, space *types.Space) *PromptTemplate {
	template := pm.NewTemplate("rag", lang)

	// 设置中间部分
	body := ""

	// 1. Space 自定义 BasePrompt（如果有）
	if space != nil && space.BasePrompt != "" {
		template.Header = space.BasePrompt + "\n\n"
	}

	// 2. RAG Prompt 模板（始终包含）
	if lang == MODEL_BASE_LANGUAGE_EN {
		body += GENERATE_PROMPT_TPL_EN
	} else {
		body += GENERATE_PROMPT_TPL_CN
	}

	template.SetBody(body)
	return template
}

// GetSummaryTemplate 获取总结场景的模板
func (pm *PromptManager) GetSummaryTemplate(lang string) *PromptTemplate {
	template := pm.NewTemplate("summary", lang)

	// 设置中间部分（配置优先）
	body := ""
	if pm.config != nil && pm.config.ChatSummary != "" {
		body = pm.config.ChatSummary
	} else {
		if lang == MODEL_BASE_LANGUAGE_EN {
			body = PROMPT_SUMMARY_DEFAULT_EN
		} else {
			body = PROMPT_SUMMARY_DEFAULT_CN
		}
	}

	template.SetBody(body)
	return template
}

// GetEnhanceQueryTemplate 获取查询增强场景的模板
func (pm *PromptManager) GetEnhanceQueryTemplate(lang string) *PromptTemplate {
	template := pm.NewTemplate("enhance_query", lang)

	// 设置中间部分（配置优先）
	body := ""
	if pm.config != nil && pm.config.EnhanceQuery != "" {
		body = pm.config.EnhanceQuery
	} else {
		if lang == MODEL_BASE_LANGUAGE_EN {
			body = PROMPT_ENHANCE_QUERY_EN
		} else {
			body = PROMPT_ENHANCE_QUERY_CN
		}
	}

	template.SetBody(body)
	return template
}

// GetButlerTemplate 获取 Butler 场景的模板
func (pm *PromptManager) GetButlerTemplate(lang string) *PromptTemplate {
	template := pm.NewTemplate("butler", lang)
	return template
}

// GetJournalTemplate 获取 Journal 场景的模板
func (pm *PromptManager) GetJournalTemplate(lang string) *PromptTemplate {
	template := pm.NewTemplate("journal", lang)
	return template
}

// GetRAGToolResponseTemplate 获取 RAG 工具响应的模板
// 用于 SearchUserKnowledges 工具返回给 AI 的内容格式
// docs: 检索到的知识文档列表，会自动转换为 prompt 文本并设置到模板中
func (pm *PromptManager) GetRAGToolResponseTemplate(lang string, docs []*types.PassageInfo) *PromptTemplate {
	// 不需要 header 和 append，只有 body
	template := &PromptTemplate{
		Lang: lang,
		Vars: make(map[string]string),
	}

	// 设置 body
	body := ""
	if lang == MODEL_BASE_LANGUAGE_EN {
		body = PROMPT_RAG_TOOL_RESPONSE_EN
	} else {
		body = PROMPT_RAG_TOOL_RESPONSE_CN
	}

	template.SetBody(body)

	// 设置检索内容变量
	d := NewDocs(docs).ConvertPassageToPromptText(lang)
	if d == "" {
		d = "null"
	}
	template.SetVar(PROMPT_VAR_RELEVANT_PASSAGE, d)

	// 设置知识数量变量
	template.SetVar("knowledge_count", strconv.Itoa(len(docs)))

	return template
}
