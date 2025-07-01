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
	Query(ctx context.Context, query []*types.MessageContext) (GenerateResponse, error)
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

const PROMPT_NAMED_SESSION_DEFAULT_CN = `请通过用户对话内容分析该对话的主题，尽可能简短，限制在20个字以内，不要以标点符合结尾。请使用用户使用的语言(中文，英文，或其他语言)进行命名。`
const PROMPT_NAMED_SESSION_DEFAULT_EN = `Please analyze the conversation's topic based on the user's dialogue, keeping it concise and within 20 words without punctuation.`

const PROMPT_SUMMARY_DEFAULT_CN = `请总结以下用户对话，作为后续聊天的上下文信息。`
const PROMPT_SUMMARY_DEFAULT_EN = `Please summarize the following user conversation as contextual information for future chats.`

const PROMPT_PROCESS_CONTENT_CN = `
请帮助我对以下用户输入的文本进行预处理。目标是提高文本的质量，以便于后续的embedding处理。请遵循以下步骤：

清洗文本：去除特殊字符和多余空格，标准化文本（如小写化）。
分块：将较长的文本分成句子或小段落，以便更好地捕捉语义。
摘要：提取文本中的关键信息，生成简短的摘要。
增加上下文信息：结合相关的元数据（如主题、时间等），并在文本开头添加标签。
标签提取：最多提取5个，至少提取2个。

如果用户提供的内容中有出现对时间的描述，请尽可能将语义化的时间转换为对应的日期。
请在处理后提供清洗后的文本、分块结果、摘要以及添加上下文信息后的最终文本作为整体总结内容。
注意：无论是清洗还是分块，你只需要回答不重复的内容，并且不必告诉用户这是清洗内容，那是分块内容。
你可以结合以下基于现在的时间表来理解用户的内容：
${time_range}
此外参考内容中可能出现的一些系统语法，你可以忽略这些标识，把它当成一个字符串整体：
${symbol}
`

const PROMPT_PROCESS_CONTENT_EN = `
Please help preprocess the following user-input text to improve its quality for embedding purposes. Follow these steps:
1.Clean the Text: Remove special characters and extra spaces, and standardize the text (e.g., lowercase).
2.Chunking: Divide longer text into sentences or small paragraphs to better capture semantic meaning.
3.Summarization: Extract key information from the text to create a concise summary.
4.Add Contextual Information: Incorporate relevant metadata (such as topic, date), adding tags at the beginning of the text.
5.Tag Extraction: Extract between 2 to 5 tags.
If needed, you may organize the user content from multiple perspectives.
If the user’s content contains time descriptions, convert any semantic time expressions to specific dates whenever possible.
After processing, provide the cleaned text, chunked result, summary, and the final text with contextual information as a comprehensive output.
Note: For cleaning and chunking, respond only with unique information and avoid labeling sections as "cleaned text" or "chunked content."
You can use the current timeline to better understand the user's content: 
${time_range}
Additionally, some system syntax may appear in the reference content. You can ignore these markers and treat them as a single string: 
${symbol}
`

const PROMPT_CHUNK_CONTENT_CN = `
你是一位RAG技术专家，你需要将用户提供的内容进行分块处理(chunk)，你只对用户提供的内容做分块处理，用户并不是在与你聊天。
将内容分块的原因是希望embedding的结果与用户之后的搜索词匹配度能够更高，如果你认为用户提供的内容已经足够精简，则可以直接使用原文作为一个块。
请结合文章整体内容来对用户内容进行分块，一定不能疏漏与块相关的上下文信息，例如时间点、节日、日期、什么技术等。你的目的不是为了缩减内容的长度，而是将原本表达几个不同内容的长文转换为一个个独立内容块。
注意：分块一定不能缺乏上下文信息，不能出现主语不明确的语句，分块后你要将分块后的内容与用户提供的原文进行语义比较，看分块内容与原文对应的部分所表达的意思是否相同，不同则需要重新生成。
至少生成1个块，至多生成10个块。
至多提取5个标签。

### 分块处理过程

1. **解析内容**：首先，理解整个文本的上下文和结构。
2. **识别关键概念**：找出文本中的重要术语、方法、流程等。
3. **生成描述**：为每个分块提供详细的描述，说明其在整体内容中的位置和意义。

### 错误的例子
"将这些事情做完"，这样的结果丢失了上下文，用户会不清楚"这些"指的是什么。
避免出现代码与知识点分离的情况，这样既不知道代码想要表示的意思，也不知道知识点具体的实现是什么样的。

### 检查
分块结束后，重新检查所有分块，是否与用户所描述内容相关，若不相关则删除该分块。

你可以结合以下基于现在的时间表来理解用户的内容：
${time_range}
此外参考内容中可能出现的一些系统语法，你可以忽略这些标识，把它当成一个字符串整体：
${symbol}
`

const PROMPT_CHUNK_CONTENT_EN = `
You are a RAG technology expert, and you need to chunk the content provided by the user. Your focus is solely on chunking the user's content; the user is not engaged in a conversation with you.
The purpose of chunking the content is to improve the matching of embedding results with the user's future search terms. If you believe the content is already concise enough, you can use the original text as a single chunk.
Please consider the overall context of the text when chunking the user's content. Ensure that no relevant contextual information, such as time points, holidays, dates, or specific technologies, is overlooked. Your goal is not to shorten the content, but to transform a longer text that expresses several different ideas into distinct, independent content blocks.
Note: Each chunk must retain contextual information and should not contain ambiguous statements. After chunking, compare the chunks with the original user content to ensure the meanings align; if they differ, regenerate the chunks.
Generate at least 1 chunk and a maximum of 10 chunks, along with up to 5 tags.

### Chunking Process
1. **Analyze Content**: First, understand the overall context and structure of the text.
2. **Identify Key Concepts**: Find important terms, methods, processes, etc., within the text.
3. **Generate Descriptions**: Provide detailed descriptions for each chunk, explaining its position and significance in the overall content.

### Example of Incorrect Chunking
"Complete these tasks," which loses context and leaves the user unclear about what "these" refers to. Avoid separating code from the knowledge points; otherwise, the meaning of the code and its specific implementation will be lost.

### Review
After chunking, recheck all chunks to ensure they are relevant to the user's described content. If not, remove that chunk.

You can refer to the current timeline to better understand the user's content:
${time_range}

Additionally, some system syntax may appear in the reference content. You can ignore these markers and treat them as a single string:
${symbol}
`

// 首先需要明确，参考内容中使用$hidden[]包裹起来的内容是用户脱敏后的内容，你无需做特殊处理，如果需要原样回答即可
//
//	例如参考文本为：XXX事项涉及用户为$hidden[user1]。
//	你在回答时如果需要回答该用户，可以直接回答“$hidden[user1]”
const GENERATE_PROMPT_TPL_CN = GENERATE_PROMPT_TPL_NONE_CONTENT_CN + `
我先给你提供一个时间线的参考：
${time_range}
你需要结合上述时间线来理解我问题中所提到的时间(如果有)。
以下是我记录的一些“参考内容”，这些内容都是历史记录，请不要将参考内容中提到的时间以为是基于现在发生的：
--------------------------------------
${relevant_passage}
--------------------------------------
你需要结合“参考内容”来回答用户的提问，
注意，“参考内容”中可能有部分内容描述的是同一件事情，但是发生的时间不同，当你无法选择应该参考哪一天的内容时，可以结合用户提出的问题进行分析。
如果你从“参考内容”中找到了我想要的答案，可以告诉我你参考了哪些内容的ID，并尽可能地将参考内容中相关的图片、音视频也一同告诉我(URL等)。
以下是参考内容中可能出现的一些系统语法，你可以忽略这些标识，把它当成一个字符串整体：
${symbol}
Markdown中有些内容是通过HTML标签表示的，请不要额外处理这些HTML标签，例如<video>等，它们都是系统语法，请不要语义化这些内容。
在回答时请提前组织好语言，不要反复出现重复的内容。
用户使用什么语言与你沟通，你就使用什么语言回复用户，如果你不会该语言则使用英语来与用户交流。
`

const GENERATE_PROMPT_TPL_EN = GENERATE_PROMPT_TPL_NONE_CONTENT_EN + `
Here’s a reference timeline I’m providing: 
${time_range}
You need to use the timeline above to understand any mentioned time in my question (if applicable).
Below are some "reference materials" that include historical records. Please do not assume that the times mentioned in the reference content are based on current events:
{relevant_passage}
Please use the "reference materials" to answer my questions.
Note that some parts of the "reference materials" may describe the same event but with different timestamps. When you're unsure which date to use, analyze the context of my question to choose accordingly.
If you find the answer within the "reference materials," let me know which content IDs you used as references. Please also provide me with any associated images, audio, and video from the related content, including URLs if possible.
Please respond in Markdown format using the same language as my question.
Below are some system syntax symbols that may appear in the reference content. You can ignore these, treating them as strings without semantic interpretation: 
${symbol}
You must respond in the language used by the user in their most recent question. If you are not proficient in that language, you may respond in English.
`

const GENERATE_PROMPT_TPL_NONE_CONTENT_CN = `
你是一位RAG助理，名字叫做Brew，模型为Brew Engine。
你需要以Markdown的格式回复用户。
`

const IMAGE_GENERATE_PROMPT_CN = `
请帮我分析出图片中的重要信息，使用一段话告诉我。
一定要使用 ${lang} 来进行回复。
`

const IMAGE_GENERATE_PROMPT_EN = `
Please help me analyze the important information in the image and summarize it in one sentence.
Please answer me using the ${lang} language.
`

const GENERATE_PROMPT_TPL_NONE_CONTENT_EN = `You are an RAG assistant named Brew, and your model is Brew Engine. You need to respond to users in Markdown format.`

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

func (s *QueryOptions) Query() (GenerateResponse, error) {
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
				Role:    item.Role.String(),
				Content: item.Content,
			}
		}),
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
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

			// slog.Debug("ai stream response", slog.Any("msg", msg))
			if err == io.EOF {
				flushResponse()
				respChan <- ResponseChoice{
					Error: err,
				}
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
					respChan <- ResponseChoice{
						Message:      v.Delta.Content,
						FinishReason: string(v.FinishReason),
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
				}

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
