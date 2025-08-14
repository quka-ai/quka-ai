package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/pkoukk/tiktoken-go"
	"github.com/samber/lo"
	"github.com/sashabaranov/go-openai"
	goopenai "github.com/sashabaranov/go-openai"

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

func BuildPrompt(basePrompt string, lang string) string {
	sb := strings.Builder{}
	if basePrompt == "" {
		switch lang {
		case MODEL_BASE_LANGUAGE_EN:
			sb.WriteString(GENERATE_PROMPT_TPL_NONE_CONTENT_EN)
		default:
			sb.WriteString(GENERATE_PROMPT_TPL_NONE_CONTENT_CN)
		}
	} else {
		sb.WriteString(basePrompt)
	}

	sb.WriteString(ReplaceVarWithLang(basePrompt, lang))
	sb.WriteString("\n")
	switch lang {
	case MODEL_BASE_LANGUAGE_EN:
		sb.WriteString(APPEND_PROMPT_EN)
	default:
		sb.WriteString(APPEND_PROMPT_CN)
	}
	return sb.String()
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

// ConvertMessageContextToEinoMessages 将 MessageContext 转换为 eino 消息格式
func ConvertMessageContextToEinoMessages(messageContexts []*types.MessageContext) []*schema.Message {
	einoMessages := make([]*schema.Message, 0, len(messageContexts))
	
	for _, msgCtx := range messageContexts {
		einoMsg := &schema.Message{
			Content: msgCtx.Content,
		}
		
		// 转换角色
		switch msgCtx.Role {
		case types.USER_ROLE_SYSTEM:
			einoMsg.Role = schema.System
		case types.USER_ROLE_USER:
			einoMsg.Role = schema.User
		case types.USER_ROLE_ASSISTANT:
			einoMsg.Role = schema.Assistant
		case types.USER_ROLE_TOOL:
			einoMsg.Role = schema.Tool
		default:
			einoMsg.Role = schema.User
		}

		// 处理工具调用
		if len(msgCtx.ToolCalls) > 0 {
			einoMsg.ToolCalls = lo.Map(msgCtx.ToolCalls, func(item goopenai.ToolCall, _ int) schema.ToolCall {
				return schema.ToolCall{
					Type: string(item.Type),
					Function: schema.FunctionCall{
						Name:      item.Function.Name,
						Arguments: item.Function.Arguments,
					},
				}
			})
		}

		// 处理多媒体内容
		if len(msgCtx.MultiContent) > 0 {
			einoMsg.MultiContent = make([]schema.ChatMessagePart, len(msgCtx.MultiContent))
			for i, part := range msgCtx.MultiContent {
				einoMsg.MultiContent[i] = schema.ChatMessagePart{
					Type: schema.ChatMessagePartType(part.Type),
					Text: part.Text,
				}

				// 转换 ImageURL
				if part.ImageURL != nil {
					einoMsg.MultiContent[i].ImageURL = &schema.ChatMessageImageURL{
						URL:    part.ImageURL.URL,
						Detail: schema.ImageURLDetail(part.ImageURL.Detail),
					}
				}
			}
		}

		einoMessages = append(einoMessages, einoMsg)
	}
	
	return einoMessages
}
