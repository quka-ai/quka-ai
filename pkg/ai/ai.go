package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pkoukk/tiktoken-go"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"

	"github.com/quka-ai/quka-ai/pkg/mark"
	"github.com/quka-ai/quka-ai/pkg/safe"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

type ModelName struct {
	ChatModel      string `toml:"chat_model"`
	EmbeddingModel string `toml:"embedding_model"`
	RerankModel    string `toml:"rerank_model"`
}

type Query interface {
	Query(ctx context.Context, query []*types.MessageContext) (*openai.ChatCompletionResponse, error)
	QueryStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
	Lang
}

type Lang interface {
	Lang() string
}

type Enhance interface {
	EnhanceQuery(ctx context.Context, messages []openai.ChatCompletionMessage) (EnhanceQueryResult, error)
	Lang() string
}

func NewQueryOptions(ctx context.Context, driver Query, model string, query []*types.MessageContext) *QueryOptions {
	return &QueryOptions{
		ctx:     ctx,
		_driver: driver,
		model:   model,
		query:   query,
	}
}

type OptionFunc func(opts *QueryOptions)

type QueryOptions struct {
	ctx            context.Context
	_driver        Query
	query          []*types.MessageContext
	docs           []*types.PassageInfo
	prompt         string
	docsSoltName   string
	vars           map[string]string
	model          string
	enableThinking bool
	tools          []openai.Tool
}

func (s *QueryOptions) WithTools(tools []openai.Tool) *QueryOptions {
	s.tools = tools
	return s
}

func (s *QueryOptions) EnableThinking(enable bool) *QueryOptions {
	s.enableThinking = enable
	return s
}

func (s *QueryOptions) WithPrompt(prompt string) *QueryOptions {
	s.prompt = strings.TrimSpace(prompt)
	return s
}

func (s *QueryOptions) WithDocsSoltName(name string) *QueryOptions {
	s.docsSoltName = name
	return s
}

func (s *QueryOptions) WithVar(key, value string) {
	if s.vars == nil {
		s.vars = make(map[string]string)
	}

	s.vars[key] = value
}

type EnhanceOptions struct {
	ctx     context.Context
	prompt  string
	_driver Enhance
	vars    map[string]string
}

func NewEnhance(ctx context.Context, driver Enhance) *EnhanceOptions {
	opt := &EnhanceOptions{
		ctx:     ctx,
		_driver: driver,
		vars:    make(map[string]string),
	}

	opt.vars[PROMPT_VAR_TIME_RANGE] = lo.If(driver.Lang() == MODEL_BASE_LANGUAGE_CN, PROMPT_ENHANCE_QUERY_CN).Else(PROMPT_ENHANCE_QUERY_EN)
	opt.vars[PROMPT_VAR_LANG] = driver.Lang()
	opt.vars[PROMPT_VAR_HISTORIES] = "null"
	opt.vars[PROMPT_VAR_SYMBOL] = CurrentSymbols
	opt.vars[PROMPT_VAR_SITE_TITLE] = SITE_TITLE

	return opt
}

func (s *EnhanceOptions) WithPrompt(prompt string) *EnhanceOptions {
	s.prompt = strings.TrimSpace(prompt)
	return s
}

func (s *EnhanceOptions) WithHistories(messages []*types.ChatMessage) *EnhanceOptions {
	if len(messages) == 0 {
		return s
	}

	str := strings.Builder{}
	for _, v := range messages {
		str.WriteString(v.Role.String())
		str.WriteString(":")
		if v.Role == types.USER_ROLE_ASSISTANT && len([]rune(v.Message)) > 40 {
			str.WriteString(string([]rune(v.Message)[:40]))
			str.WriteString("......")
		} else {
			str.WriteString(v.Message)
		}
		str.WriteString("\n")
	}

	s.vars[PROMPT_VAR_HISTORIES] = str.String()
	return s
}

// const PROMPT_ENHANCE_QUERY_CN = `任务指令：作为查询增强器，你的目标是通过增加相关信息来提高用户查询的相关性和多样性。请根据提供的指导原则对用户的原始查询进行优化。 参考信息：
// - 时间表：
// ${time_range}
// - 如果用户提到时间，请依据上述时间表将模糊的时间描述转换为具体的日期。
// - 如果用户提及地点，请确保在增强后的查询中包含该位置信息。
// - 对于一些通用表达（如“干啥”），请使用其同义词或更正式的表述（例如，“做什么”）来进行替换。
// 操作指南：
// 1. 保持用户原始查询的核心意图不变。
// 2. 尽可能简短地添加额外的信息到用户的查询中，而不是替换原有的内容。
// 3. 目标是生成一个更加具体、相关性更高的查询版本，以帮助获取更多相似的问题或答案。
// 示例处理流程：
// - 用户输入：“周末有什么活动？”
// - 增强后输出：“${time_range}中的具体周末有哪些活动？”
// 注意事项：
// - 确保最终输出既保留了用户的原意，又增加了有助于搜索的相关细节。
// - 不要改变用户提问的基本结构，仅在其基础上做必要的补充和调整。
// 请基于以上规则告诉我经过处理后的用户语句，注意，我会直接使用你处理后的语句来进行RAG流程的下一步，请不要在响应中添加任何与任务无关的内容。`

const PROMPT_ENHANCE_QUERY_CN = `
## 你的任务
你作为一个向量检索助手，你的任务是结合历史记录，从不同角度，为“原问题”生成个不同版本的“检索词”，从而提高向量检索的语义丰富度，提高向量检索的精度。
生成的问题要求指向对象清晰明确，并与“原问题语言相同”。

## 基于我现在的时间线参考
${time_range}

## 参考示例

历史记录: 
"""
null
"""
原问题: 介绍下剧情。
检索词: ["介绍下故事的背景。","故事的主题是什么？","介绍下故事的主要人物。"]
----------------
历史记录: 
"""
user: 对话背景。
assistant: 当前对话是关于 Nginx 的介绍和使用等。
"""
原问题: 怎么下载
检索词: ["Nginx 如何下载？","下载 Nginx 需要什么条件？","有哪些渠道可以下载 Nginx？"]
----------------
历史记录: 
"""
user: 对话背景。
assistant: 当前对话是关于 Nginx 的介绍和使用等。
user: 报错 "no connection"
assistant: 报错"no connection"可能是因为……
"""
原问题: 怎么解决
检索词: ["Nginx报错"no connection"如何解决？","造成'no connection'报错的原因。","Nginx提示'no connection'，要怎么办？"]
----------------
历史记录: 
"""
user: How long is the maternity leave?
assistant: The number of days of maternity leave depends on the city in which the employee is located. Please provide your city so that I can answer your questions.
"""
原问题: ShenYang
检索词: ["How many days is maternity leave in Shenyang?","Shenyang's maternity leave policy.","The standard of maternity leave in Shenyang."]
----------------
历史记录: 
"""
user: 作者是谁？
assistant: ${title} 的作者是 boyce。
"""
原问题: Tell me about him
检索词: ["Introduce labring, the author of ${title}." ," Background information on author boyce." "," Why does boyce do ${title}?"]
----------------
历史记录:
"""
user: 对话背景。
assistant: 关于 ${title} 的介绍和使用等问题。
"""
原问题: 你好。
检索词: ["你好"]
----------------
历史记录:
"""
null
"""
原问题: 我昨天干啥了？
检索词: ["我昨天做了哪些事情","昨天{参考时间表获取对应日期}做了什么事情"]
----------------

## 输出要求

1. 输出格式为 JSON 数组，不需要使用markdown语法，数组中每个元素为字符串。无需对输出进行任何解释。
2. 输出语言与原问题相同。原问题为中文则输出中文；原问题为英文则输出英文。

## 开始任务

历史记录:
"""
${histories}
"""
原问题: ${query}
检索词: 
`

const PROMPT_ENHANCE_QUERY_EN = `You are a query enhancer. You must enhance the user's statements to make them more relevant to the content the user might be searching for. You can refer to the following timeline to understand the user's question:
${time_range}
If the user mentions time, you can replace the time description with specific dates based on the provided reference timeline. If any locations are mentioned, please add them to the query as well. You need to perform synonym transformations on some common phrases in the user's query, such as "干啥" can also be described as "做什么." Keep your responses as brief as possible. Add to the user's query without replacing it.`

func (s *EnhanceOptions) EnhanceQuery(query string) (EnhanceQueryResult, error) {
	if s.prompt == "" {
		switch s._driver.Lang() {
		case MODEL_BASE_LANGUAGE_CN:
			s.prompt = PROMPT_ENHANCE_QUERY_CN
		default:
			s.prompt = PROMPT_ENHANCE_QUERY_EN
		}
	}

	for k, v := range s.vars {
		s.prompt = strings.ReplaceAll(s.prompt, k, v)
	}

	s.prompt = strings.ReplaceAll(s.prompt, PROMPT_VAR_QUERY, query)

	res, err := s._driver.EnhanceQuery(s.ctx, []openai.ChatCompletionMessage{
		{
			Role:    types.USER_ROLE_USER.String(),
			Content: s.prompt,
		},
	})
	if err != nil {
		return res, err
	}

	res.Original = query
	return res, nil
}

func (s *QueryOptions) Query() (*openai.ChatCompletionResponse, error) {
	if s.prompt == "" {
		switch s._driver.Lang() {
		case MODEL_BASE_LANGUAGE_CN:
			s.prompt = GENERATE_PROMPT_TPL_NONE_CONTENT_CN
		default:
			s.prompt = GENERATE_PROMPT_TPL_NONE_CONTENT_EN
		}
	}

	s.prompt = ReplaceVarWithLang(s.prompt, s._driver.Lang())
	for k, v := range s.vars {
		s.prompt = strings.ReplaceAll(s.prompt, k, v)
	}

	s.prompt += APPEND_PROMPT_CN

	if len(s.query) > 0 && s.query[0].Role != types.USER_ROLE_SYSTEM {
		s.query = append([]*types.MessageContext{
			{
				Role:    types.USER_ROLE_SYSTEM,
				Content: s.prompt,
			},
		}, s.query...)
	} else if len(s.query) == 0 {
		s.query = []*types.MessageContext{
			{
				Role:    types.USER_ROLE_SYSTEM,
				Content: s.prompt,
			},
		}
	}

	return s._driver.Query(s.ctx, s.query)
}

func (s *QueryOptions) QueryStream() (*openai.ChatCompletionStream, error) {
	if s.prompt == "" {
		switch s._driver.Lang() {
		case MODEL_BASE_LANGUAGE_CN:
			s.prompt = GENERATE_PROMPT_TPL_NONE_CONTENT_CN
		default:
			s.prompt = GENERATE_PROMPT_TPL_NONE_CONTENT_EN
		}
	}

	s.prompt = ReplaceVarWithLang(s.prompt, s._driver.Lang())
	for k, v := range s.vars {
		s.prompt = strings.ReplaceAll(s.prompt, k, v)
	}

	s.prompt += APPEND_PROMPT_CN

	if len(s.query) > 0 {
		if s.query[0].Role != types.USER_ROLE_SYSTEM {
			s.query = append([]*types.MessageContext{
				{
					Role:    types.USER_ROLE_SYSTEM,
					Content: s.prompt,
				},
			}, s.query...)
		}
	} else {
		s.query = []*types.MessageContext{
			{
				Role:    types.USER_ROLE_SYSTEM,
				Content: s.prompt,
			},
		}
	}

	req := openai.ChatCompletionRequest{
		Model:  s.model,
		Stream: true,
		ChatTemplateKwargs: map[string]any{
			"enable_thinking": s.enableThinking,
		},
		Messages: lo.Map(s.query, func(item *types.MessageContext, _ int) openai.ChatCompletionMessage {
			return openai.ChatCompletionMessage{
				Role:         item.Role.String(),
				Content:      item.Content,
				MultiContent: item.MultiContent,
			}
		}),
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
		Tools: s.tools,
	}

	return s._driver.QueryStream(s.ctx, req)
}

func HandleAIStream(ctx context.Context, resp *openai.ChatCompletionStream, marks map[string]string) (chan ResponseChoice, error) {
	respChan := make(chan ResponseChoice, 10)
	ticker := time.NewTicker(time.Millisecond * 500)
	go safe.Run(func() {
		ctx, cancel := context.WithCancel(ctx)
		defer func() {
			close(respChan)
			resp.Close()
			ticker.Stop()
			cancel()
		}()

		var (
			once      = sync.Once{}
			strs      = strings.Builder{}
			messageID string
			mu        sync.Mutex

			maybeMarks  bool
			machedMarks bool
			needToMarks = len(marks) > 0

			startThinking    = sync.Once{}
			hasThinking      = false
			finishedThinking = sync.Once{}

			toolCalls []*openai.ToolCall
		)

		flushResponse := func() {
			mu.Lock()
			defer mu.Unlock()
			if strs.Len() > 0 {
				respChan <- ResponseChoice{
					ID:      messageID,
					Message: strs.String(),
				}
				strs.Reset()
			}
		}

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if maybeMarks {
						continue
					}
					flushResponse()
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				respChan <- ResponseChoice{
					Error: ctx.Err(),
				}
				return
			default:
			}

			msg, err := resp.Recv()
			if err != nil && err != io.EOF {
				respChan <- ResponseChoice{
					Error: err,
				}
				return
			}

			// raw, _ := json.Marshal(msg)
			// fmt.Println("inner msg", string(raw))

			// slog.Debug("ai stream response", slog.Any("msg", msg))
			if err == io.EOF {
				flushResponse()
				return
			}

			// slog.Debug("message usage", slog.Any("msg", msg))
			if msg.Usage != nil {
				respChan <- ResponseChoice{
					Usage: msg.Usage,
					Model: msg.Model,
				}
			}

			for _, v := range msg.Choices {
				if v.FinishReason != "" {
					if strs.Len() > 0 {
						flushResponse()
					}
					if v.FinishReason == "tool_calls" {
						respChan <- ResponseChoice{
							DeepContinue: toolCalls,
						}
					}
					respChan <- ResponseChoice{
						Message:      v.Delta.Content,
						FinishReason: string(v.FinishReason),
					}
				}

				if len(v.Delta.ToolCalls) > 0 {
					for _, toolCall := range v.Delta.ToolCalls {
						if len(toolCalls) == 0 {
							toolCalls = append(toolCalls, &toolCall)
							continue
						}
						var toolCallIndex int
						if toolCall.Index != nil {
							toolCallIndex = *toolCall.Index
						}
						toolCalls[toolCallIndex].Function.Name += toolCall.Function.Name
						toolCalls[toolCallIndex].Function.Arguments += toolCall.Function.Arguments
					}
				}

				if v.Delta.Content == "" && v.Delta.ReasoningContent == "" {
					continue
				}
				if needToMarks {
					if !maybeMarks {
						if strings.Contains(v.Delta.Content, "$") {
							maybeMarks = true
							if strs.Len() != 0 {
								flushResponse()
							}
						}
					} else if maybeMarks && strs.Len() >= 10 {
						if strings.Contains(strs.String(), "$hidden[") {
							machedMarks = true
						} else {
							maybeMarks = false
						}
					}
				}

				if len(v.Delta.ReasoningContent) > 0 {
					startThinking.Do(func() {
						hasThinking = true
						if strings.Contains(v.Delta.ReasoningContent, "\n") {
							v.Delta.ReasoningContent = ""
						}
						strs.WriteString("<think>")
					})
					strs.WriteString(strings.ReplaceAll(v.Delta.ReasoningContent, "\n", "</br>"))
				}

				if len(v.Delta.Content) > 0 {
					finishedThinking.Do(func() {
						if !hasThinking {
							return
						}
						strs.WriteString("</think>")
					})
					strs.WriteString(v.Delta.Content)

					if machedMarks && strings.Contains(v.Delta.Content, "]") {
						text, replaced := mark.ResolveHidden(strs.String(), func(fakeValue string) string {
							real := marks[fakeValue]
							return real
						}, false)
						if replaced {
							strs.Reset()
							strs.WriteString(text)
							maybeMarks = false
							machedMarks = false
						}
					}
				}

				once.Do(func() {
					messageID = msg.ID
					// flushResponse() // 快速响应出去
				})
			}
		}
	})
	return respChan, nil
}

const (
	MODEL_BASE_LANGUAGE_CN = "CN"
	MODEL_BASE_LANGUAGE_EN = "EN"
)

func BuildRAGPrompt(tpl string, docs Docs, driver Lang) string {
	if tpl == "" {
		switch driver.Lang() {
		case MODEL_BASE_LANGUAGE_CN:
			tpl = GENERATE_PROMPT_TPL_CN
		default:
			tpl = GENERATE_PROMPT_TPL_EN
		}
	}

	tpl = ReplaceVarWithLang(tpl, driver.Lang())

	d := docs.ConvertPassageToPromptText(driver.Lang())
	if d == "" {
		d = "null"
	}
	tpl = strings.ReplaceAll(tpl, PROMPT_VAR_RELEVANT_PASSAGE, d)

	tpl += APPEND_PROMPT_CN
	return tpl
}

func ReplaceVarWithLang(tpl, lang string) string {
	switch lang {
	case MODEL_BASE_LANGUAGE_CN:
		tpl = ReplaceVarCN(tpl)
	default:
		tpl = ReplaceVarEN(tpl)
	}
	return tpl
}

func ReplaceVarCN(tpl string) string {
	tpl = strings.ReplaceAll(tpl, PROMPT_VAR_TIME_RANGE, GenerateTimeListAtNowCN())
	tpl = strings.ReplaceAll(tpl, PROMPT_VAR_SYMBOL, CurrentSymbols)
	return tpl
}

func ReplaceVarEN(tpl string) string {
	tpl = strings.ReplaceAll(tpl, PROMPT_VAR_TIME_RANGE, GenerateTimeListAtNowEN())
	tpl = strings.ReplaceAll(tpl, PROMPT_VAR_SYMBOL, CurrentSymbols)
	return tpl
}

type Docs interface {
	ConvertPassageToPromptText(lang string) string
}

type docs struct {
	docs []*types.PassageInfo
}

func (d *docs) ConvertPassageToPromptText(lang string) string {
	switch lang {
	case MODEL_BASE_LANGUAGE_CN:
		return convertPassageToPromptTextCN(d.docs)
	default:
		return convertPassageToPromptTextEN(d.docs)
	}
}

func NewDocs(list []*types.PassageInfo) Docs {
	return &docs{
		docs: list,
	}
}

func convertPassageToPromptTextCN(docs []*types.PassageInfo) string {
	s := strings.Builder{}
	for i, v := range docs {
		if v.Content == "" {
			continue
		}
		if i != 0 {
			s.WriteString("------\n")
		}
		s.WriteString("这件事发生在：")
		s.WriteString(v.DateTime)
		s.WriteString("\n")
		if v.ID != "" {
			s.WriteString("ID：")
			s.WriteString(v.ID)
			s.WriteString("\n")
		}
		if v.Resource != "" {
			s.WriteString("内容类型：")
			s.WriteString(v.Resource)
			s.WriteString("\n")
		}
		s.WriteString("内容：")
		s.WriteString(v.Content)
		s.WriteString("\n")
	}

	return s.String()
}

func convertPassageToPromptTextEN(docs []*types.PassageInfo) string {
	s := strings.Builder{}
	for i, v := range docs {
		if i != 0 {
			s.WriteString("------\n")
		}
		s.WriteString("Event Time：")
		s.WriteString(v.DateTime)
		s.WriteString("\n")
		s.WriteString("ID：")
		s.WriteString(v.ID)
		s.WriteString("\n")
		s.WriteString("Resource Kind：")
		s.WriteString(v.Resource)
		s.WriteString("\nContent：")
		s.WriteString(v.Content)
		s.WriteString("\n")
	}

	return s.String()
}

func convertPassageToPrompt(docs []*types.PassageInfo) string {
	raw, _ := json.MarshalIndent(docs, "", "  ")
	b := strings.Builder{}
	b.WriteString("``` json\n")
	b.Write(raw)
	b.WriteString("\n")
	b.WriteString("```\n")
	return b.String()
}

type GenerateResponse struct {
	Received []string      `json:"received"`
	Usage    *openai.Usage `json:"-"`
	Model    string        `json:"model"`
}

func (r GenerateResponse) Message() string {
	b := strings.Builder{}

	for i, item := range r.Received {
		if i != 0 {
			b.WriteString("\n")
		}
		b.WriteString(item)
	}

	return b.String()
}

type SummarizeResult struct {
	Title    string        `json:"title"`
	Tags     []string      `json:"tags"`
	Summary  string        `json:"summary"`
	DateTime string        `json:"date_time"`
	Usage    *openai.Usage `json:"-"`
	Model    string        `json:"model"`
}

type ChunkResult struct {
	Title    string        `json:"title"`
	Tags     []string      `json:"tags"`
	Chunks   []string      `json:"chunks"`
	DateTime string        `json:"date_time"`
	Usage    *openai.Usage `json:"-"`
	Model    string        `json:"model"`
}

type EmbeddingResult struct {
	Model string
	Usage *openai.Usage
	Data  [][]float32
}

type EnhanceQueryResult struct {
	Original string        `json:"original"`
	News     []string      `json:"news"`
	Model    string        `json:"model"`
	Usage    *openai.Usage `json:"-"`
}

func (e EnhanceQueryResult) ResultQuery() string {
	b := strings.Builder{}
	b.WriteString(e.Original)
	for i, item := range e.News {
		if i != 0 {
			b.WriteString(" ")
		}
		b.WriteString(item)
	}

	return b.String()
}

type UsageItem struct {
	Subject string
	Usage   Usage
}

type Usage struct {
	Model string        `json:"model"`
	Usage *openai.Usage `json:"-"`
}

const (
	DEFAULT_TIME_TPL_FORMAT = "2006-01-02 15:04"
	DEFAULT_DATE_TPL_FORMAT = "2006-01-02"
)

func timeFormat(t time.Time) string {
	return t.Local().Format(DEFAULT_TIME_TPL_FORMAT)
}

func dateFormat(t time.Time) string {
	return t.Local().Format(DEFAULT_DATE_TPL_FORMAT)
}

type RerankDoc struct {
	ID      string
	Content string
}

type RankDocItem struct {
	ID    string
	Score float64
}

// TODO i18n
func GenerateTimeListAtNowCN() string {
	now := time.Now()

	tpl := strings.Builder{}
	tpl.WriteString("现在(今天)是：")
	tpl.WriteString(timeFormat(now))
	tpl.WriteString("，星期：")
	var week string
	switch now.Weekday() {
	case time.Monday:
		week = "一"
	case time.Tuesday:
		week = "二"
	case time.Wednesday:
		week = "三"
	case time.Thursday:
		week = "四"
	case time.Friday:
		week = "五"
	case time.Saturday:
		week = "六"
	case time.Sunday:
		week = "日"
	}
	tpl.WriteString(week)
	tpl.WriteString("\n")

	tpl.WriteString("明天：")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, 1)))
	tpl.WriteString("\n")

	tpl.WriteString("后天：")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, 2)))
	tpl.WriteString("\n")

	tpl.WriteString("大后天：")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, 3)))
	tpl.WriteString("\n")

	tpl.WriteString("昨天：")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, -1)))
	tpl.WriteString("\n")

	tpl.WriteString("前天：")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, -2)))
	tpl.WriteString("\n")

	tpl.WriteString("大前天：")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, -3)))
	tpl.WriteString("\n")

	tpl.WriteString("本周的起止范围是：")
	wst, wet := utils.GetWeekStartAndEnd(now)
	tpl.WriteString(dateFormat(wst))
	tpl.WriteString(" 至 ")
	tpl.WriteString(dateFormat(wet))
	tpl.WriteString("\n")

	tpl.WriteString("下周的起止范围是：")
	wst, wet = utils.GetWeekStartAndEnd(now.AddDate(0, 0, 7))
	tpl.WriteString(dateFormat(wst))
	tpl.WriteString(" 至 ")
	tpl.WriteString(dateFormat(wet))
	tpl.WriteString("\n")

	tpl.WriteString("上周的起止范围是：")
	wst, wet = utils.GetWeekStartAndEnd(now.AddDate(0, 0, -7))
	tpl.WriteString(dateFormat(wst))
	tpl.WriteString(" 至 ")
	tpl.WriteString(dateFormat(wet))
	tpl.WriteString("\n")

	tpl.WriteString("本月的起止范围是：")
	mst, met := utils.GetMonthStartAndEnd(now)
	tpl.WriteString(dateFormat(mst))
	tpl.WriteString(" 至 ")
	tpl.WriteString(dateFormat(met))
	tpl.WriteString("\n")

	tpl.WriteString("下月的起止范围是：")
	mst, met = utils.GetMonthStartAndEnd(now.AddDate(0, 1, 0))
	tpl.WriteString(dateFormat(mst))
	tpl.WriteString(" 至 ")
	tpl.WriteString(dateFormat(met))
	tpl.WriteString("\n")

	tpl.WriteString("上月的起止范围是：")
	mst, met = utils.GetMonthStartAndEnd(now.AddDate(0, -1, 0))
	tpl.WriteString(dateFormat(mst))
	tpl.WriteString(" 至 ")
	tpl.WriteString(dateFormat(met))

	return tpl.String()
}

func GenerateTimeListAtNowEN() string {
	now := time.Now()

	tpl := strings.Builder{}
	tpl.WriteString("Today is：")
	tpl.WriteString(timeFormat(now))
	tpl.WriteString(" ")
	tpl.WriteString(now.Weekday().String())
	tpl.WriteString("\n")

	tpl.WriteString("Tomorrow:")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, 1)))
	tpl.WriteString("\n")

	tpl.WriteString("The day after tomorrow: ")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, 2)))
	tpl.WriteString("\n")

	tpl.WriteString("Two days after tomorrow: ")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, 3)))
	tpl.WriteString("\n")

	tpl.WriteString("Yesterday: ")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, -1)))
	tpl.WriteString("\n")

	tpl.WriteString("The day before yesterday: ")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, -2)))
	tpl.WriteString("\n")

	tpl.WriteString("Two day before yesterday: ")
	tpl.WriteString(dateFormat(now.AddDate(0, 0, -3)))
	tpl.WriteString("\n")

	tpl.WriteString("The start and end range of this week is from: ")
	wst, wet := utils.GetWeekStartAndEnd(now)
	tpl.WriteString(dateFormat(wst))
	tpl.WriteString(" to ")
	tpl.WriteString(dateFormat(wet))
	tpl.WriteString("\n")

	tpl.WriteString("The start and end range of next week is from: ")
	wst, wet = utils.GetWeekStartAndEnd(now.AddDate(0, 0, 7))
	tpl.WriteString(dateFormat(wst))
	tpl.WriteString(" to ")
	tpl.WriteString(dateFormat(wet))
	tpl.WriteString("\n")

	tpl.WriteString("The start and end range of last week is from: ")
	wst, wet = utils.GetWeekStartAndEnd(now.AddDate(0, 0, -7))
	tpl.WriteString(dateFormat(wst))
	tpl.WriteString(" to ")
	tpl.WriteString(dateFormat(wet))
	tpl.WriteString("\n")

	tpl.WriteString("The start and end range of this month is from: ")
	mst, met := utils.GetMonthStartAndEnd(now)
	tpl.WriteString(dateFormat(mst))
	tpl.WriteString(" to ")
	tpl.WriteString(dateFormat(met))
	tpl.WriteString("\n")

	tpl.WriteString("The start and end range of next month is from: ")
	mst, met = utils.GetMonthStartAndEnd(now.AddDate(0, 1, 0))
	tpl.WriteString(dateFormat(mst))
	tpl.WriteString(" to ")
	tpl.WriteString(dateFormat(met))
	tpl.WriteString("\n")

	tpl.WriteString("The start and end range of last month is from: ")
	mst, met = utils.GetMonthStartAndEnd(now.AddDate(0, -1, 0))
	tpl.WriteString(dateFormat(mst))
	tpl.WriteString(" to ")
	tpl.WriteString(dateFormat(met))

	return tpl.String()
}

type MessageContext = openai.ChatCompletionMessage
type ResponseChoice struct {
	ID           string
	Message      string
	FinishReason string
	Error        error
	Usage        *openai.Usage
	Model        string
	DeepContinue []*openai.ToolCall
}

type ReaderResult struct {
	Warning     string `json:"warning"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Url         string `json:"url"`
	Content     string `json:"content"`
	Usage       struct {
		Tokens int `json:"tokens"`
	} `json:"usage"`
}

func NumTokens(messages []openai.ChatCompletionMessage, model string) (numTokens int, err error) {
	var tokensPerMessage, tokensPerName int
	switch model {
	case "gpt-3.5-turbo-0613",
		"gpt-3.5-turbo-16k-0613",
		"gpt-4-0314",
		"gpt-4-32k-0314",
		"gpt-4-0613",
		"gpt-4-32k-0613":
		tokensPerMessage = 3
		tokensPerName = 1
	case "gpt-3.5-turbo-0301":
		tokensPerMessage = 4 // every message follows <|start|>{role/name}\n{content}<|end|>\n
		tokensPerName = -1   // if there's a name, the role is omitted
	default:
		if strings.Contains(model, "gpt-4") {
			return NumTokens(messages, "gpt-4-0613")
		} else {
			return NumTokens(messages, "gpt-3.5-turbo-0613")
		}
	}

	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		err = fmt.Errorf("encoding for model: %v", err)
		return
	}

	for _, message := range messages {
		numTokens += tokensPerMessage
		numTokens += len(tkm.Encode(message.Content, nil, nil))
		numTokens += len(tkm.Encode(message.Role, nil, nil))
		numTokens += len(tkm.Encode(message.Name, nil, nil))
		if message.Name != "" {
			numTokens += tokensPerName
		}
	}
	numTokens += 3 // every reply is primed with <|start|>assistant<|message|>
	return numTokens, nil
}
