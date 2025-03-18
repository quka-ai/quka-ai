package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/types"
)

const (
	NAME = "gemini"
)

type Driver struct {
	client *genai.Client
}

func New(token string) *Driver {
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(token))
	if err != nil {
		panic(err)
	}

	return &Driver{
		client: client,
	}
}

func (s *Driver) Lang() string {
	return ai.MODEL_BASE_LANGUAGE_EN
}

func (s *Driver) embedding(ctx context.Context, title, content string) ([]float32, error) {
	slog.Debug("Embedding", slog.String("driver", NAME))
	em := s.client.EmbeddingModel("embedding-001")
	if title != "" {
		em.TaskType = genai.TaskTypeRetrievalDocument
	} else {
		em.TaskType = genai.TaskTypeRetrievalQuery
	}

	res, err := em.EmbedContentWithTitle(ctx, title, genai.Text(content))
	if err != nil {
		return nil, err
	}
	return res.Embedding.Values, nil
}

func (s *Driver) EmbeddingForQuery(ctx context.Context, content string) ([]float32, error) {
	return s.embedding(ctx, "", content)
}

func (s *Driver) EmbeddingForDocument(ctx context.Context, title, content string) ([]float32, error) {
	return s.embedding(ctx, title, content)
}

const GENERATE_PROMPT_TPL = `
  You are a helpful and informative bot that answers questions using text from the reference passage included below. 
  Be sure to respond in a complete sentence, being comprehensive, including all relevant background information. 
  However, you are talking to a non-technical audience, so be sure to break down complicated concepts and 
  strike a friendly and converstional tone. 
  If the passage is irrelevant to the answer, you may ignore it.
  QUESTION: '{query}'
  PASSAGE: 
  '{relevant_passage}'

  Please use {lang} to ANSWER:
`

const GENERATE_PROMPT_TPL_CN = `
    以下是关于回答用户提问可以参考的内容(json格式)：
    --------------------------------------
	{relevant_passage}
    --------------------------------------
    你需要结合“参考内容”来回答用户的提问，如果参考内容完全没有用户想要的结果，你再通过自己的知识进行回答。
    如果你从上述内容中找到了用户想要的答案，可以结合内容相关的属性来给到用户更多的帮助，比如告诉用户你参考了ID为XXX的文章。
    请你使用 {lang} 语言，以Markdown格式回复用户。
`

func convertPassageToPrompt(docs []*types.PassageInfo) string {
	raw, _ := json.MarshalIndent(docs, "", "  ")
	b := strings.Builder{}
	b.WriteString("``` json\n")
	b.Write(raw)
	b.WriteString("\n")
	b.WriteString("```\n")
	return b.String()
}

func (s *Driver) Query(ctx context.Context, query string, docs []*types.PassageInfo) (ai.GenerateResponse, error) {
	prompt := strings.ReplaceAll(GENERATE_PROMPT_TPL_CN, "{query}", query)
	prompt = strings.ReplaceAll(prompt, "{relevant_passage}", convertPassageToPrompt(docs))

	model := s.client.GenerativeModel("gemini-1.5-pro-latest")
	model.SystemInstruction = genai.NewUserContent(genai.Text(prompt))
	// Ask the model to respond with JSON.
	model.ResponseMIMEType = "application/json"
	// Specify the schema.
	model.ResponseSchema = &genai.Schema{
		Type:  genai.TypeArray,
		Items: &genai.Schema{Type: genai.TypeString},
	}

	slog.Debug("Query", slog.String("prompt", prompt), slog.String("query", query), slog.String("driver", NAME))

	var result ai.GenerateResponse
	resp, err := model.GenerateContent(ctx, genai.Text(query))
	if err != nil {
		return result, err
	}

	if len(resp.Candidates) == 0 {
		return result, errors.New("empty response content")
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if resp.Candidates[0].FinishReason != genai.FinishReasonStop {
			slog.Warn("ProcessContent, ai finished without stop", slog.String("reason", resp.Candidates[0].FinishReason.String()))
		}
		if txt, ok := part.(genai.Text); ok {
			var recipes []string
			if err := json.Unmarshal([]byte(txt), &recipes); err != nil {
				return result, fmt.Errorf("failed to unmarshal ai response content, %w", err)
			}
			result.Received = append(result.Received, recipes...)
		}
	}

	result.Usage = &openai.Usage{
		PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
		CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
		TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
	}

	return result, nil
}

const PROMPT_PROCESS_CONTENT_CN = `
请按照下列内容对文本进行处理：
1. 分词
2. 去除停用词
3. 词形还原
4. 命名实体识别

请结合上述几点处理后的内容，生成摘要与标签反馈给用户。
标签最多提取5个。

请按照这个流程处理用户提供的文本。
`

func (s *Driver) ProcessContent(ctx context.Context, doc *string) (ai.GenerateResponse, error) {
	model := s.client.GenerativeModel("gemini-1.5-pro-latest")
	model.SystemInstruction = genai.NewUserContent(genai.Text(PROMPT_PROCESS_CONTENT_CN))
	// Ask the model to respond with JSON.
	model.ResponseMIMEType = "application/json"
	// Specify the schema.
	model.ResponseSchema = &genai.Schema{
		Type:  genai.TypeArray,
		Items: &genai.Schema{Type: genai.TypeString},
	}

	slog.Debug("ProcessContent", slog.String("driver", "gemini"), slog.String("prompt", PROMPT_PROCESS_CONTENT_CN), slog.String("doc", *doc))

	var result ai.GenerateResponse
	resp, err := model.GenerateContent(ctx, genai.Text(*doc))
	if err != nil {
		return result, err
	}

	if len(resp.Candidates) == 0 {
		return result, errors.New("empty response content")
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if resp.Candidates[0].FinishReason != genai.FinishReasonStop {
			slog.Warn("ProcessContent, ai finished without stop", slog.String("reason", resp.Candidates[0].FinishReason.String()))
		}
		if txt, ok := part.(genai.Text); ok {
			var recipes []string
			if err := json.Unmarshal([]byte(txt), &recipes); err != nil {
				return result, fmt.Errorf("failed to unmarshal ai response content, %w", err)
			}
			result.Received = append(result.Received, recipes...)
		}
	}
	result.Usage = &openai.Usage{
		PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
		CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
		TotalTokens:      int(resp.UsageMetadata.TotalTokenCount),
	}

	return result, nil
}

func (s *Driver) Summarize(ctx context.Context, doc *string) (ai.SummarizeResult, error) {
	slog.Debug("Summarize", slog.String("driver", NAME))
	// To use functions / tools, we have to first define a schema that describes
	// the function to the model. The schema is similar to OpenAPI 3.0.
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"tags": {
				Type:        genai.TypeArray,
				Description: "Extract relevant keywords or technical tags from the user's description to assist the user in categorizing related content later. Organize these values in an array format.",
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
			},
			"title": {
				Type:        genai.TypeString,
				Description: "Generate a title for the content provided by the user and fill in this field.",
			},
			"summary": {
				Type:        genai.TypeString,
				Description: "Processed summary content.",
			},
		},
		Required: []string{"tags", "title", "summary"},
	}

	summaryTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "summarize",
			Description: "Processed summary content.",
			Parameters:  schema,
		}},
	}

	model := s.client.GenerativeModel("gemini-1.5-pro-latest")

	// Before initiating a conversation, we tell the model which tools it has
	// at its disposal.
	model.Tools = []*genai.Tool{summaryTool}
	model.ToolConfig = &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingAny,
		},
	}

	model.SystemInstruction = genai.NewUserContent(genai.Text(ai.ReplaceVarCN(ai.PROMPT_PROCESS_CONTENT_EN)))
	var result ai.SummarizeResult
	// res, err := model.GenerateContent(ctx, genai.Text(*doc))
	// if err != nil {
	// 	return result, fmt.Errorf("funcall failed: %v", err)
	// }
	session := model.StartChat()

	res, err := session.SendMessage(ctx, genai.Text(*doc))
	if err != nil {
		return result, fmt.Errorf("session.SendMessage: %v", err)
	}

	part := res.Candidates[0].Content.Parts[0]
	funcall, ok := part.(genai.FunctionCall)
	if !ok || funcall.Name != "summarize" {
		return result, fmt.Errorf("expected FunctionCall to find_theaters: %v", part)
	}

	for k, v := range funcall.Args {
		// Expect the model to pass a proper string "{k}" argument to the tool.
		switch k {
		case "title":
			if parsed, ok := v.(string); ok {
				result.Title = parsed
			} else {
				return result, fmt.Errorf("funcall args parse expected string: %v", funcall.Args[k])
			}
		case "summary":
			if parsed, ok := v.(string); ok {
				result.Summary = parsed
			} else {
				return result, fmt.Errorf("funcall args parse expected string: %v", funcall.Args[k])
			}
		case "tags":
			if parsed, ok := v.([]interface{}); ok {
				for _, v := range parsed {
					if str, ok := v.(string); ok {
						result.Tags = append(result.Tags, str)
					}
				}
			} else {
				return result, fmt.Errorf("funcall args parse expected []string: %v", funcall.Args[k])
			}
		}

	}

	return result, nil
}
