package gemini_test

import (
	"context"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/quka-ai/quka-ai/pkg/ai/gemini"
	"github.com/quka-ai/quka-ai/pkg/types"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

func new() *gemini.Driver {
	return gemini.New(os.Getenv("QUKA_API_AI_GEMINI_TOKEN"))
}

func Test_Embedding(t *testing.T) {
	d := new()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	res, err := d.EmbeddingForDocument(ctx, "test", "this is test content for test embedding")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(len(res))

	assert.Greater(t, len(res), 0)
}

func Test_Generate(t *testing.T) {
	d := new()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	res, err := d.Query(ctx, "我的车昨天停哪了？", []*types.PassageInfo{
		{
			ID:      "xcjoijoijo12",
			Content: "我有一辆白色的车",
		},
		{
			ID:      "xcjoiaajoijo12",
			Content: "我有一辆白色的自行车",
		},
		{
			ID:      "xcjoij12312ijo12",
			Content: "我昨天把车停在了B2层",
		},
		{
			ID:      "3333oijoijo12",
			Content: "停车楼里有十辆车",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(res)
}

func Test_ProcessContent(t *testing.T) {
	content := "Docker 支持 64 位版本 CentOS 7/8，并且要求内核版本不低于 3.10。 CentOS 7 满足最低内核的要求，但由于内核版本比较低，部分功能（如 overlay2 存储层驱动）无法使用，并且部分功能可能不太稳定。"

	d := new()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	resp, err := d.ProcessContent(ctx, &content)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}

func Test_Summarize(t *testing.T) {
	content := "Docker 支持 64 位版本 CentOS 7/8，并且要求内核版本不低于 3.10。 CentOS 7 满足最低内核的要求，但由于内核版本比较低，部分功能（如 overlay2 存储层驱动）无法使用，并且部分功能可能不太稳定。"

	d := new()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	resp, err := d.Summarize(ctx, &content)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}

func Test_Tool(t *testing.T) {
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(os.Getenv("QUKA_API_AI_GEMINI_TOKEN")))
	if err != nil {
		t.Fatal(err)
	}
	// To use functions / tools, we have to first define a schema that describes
	// the function to the model. The schema is similar to OpenAPI 3.0.
	// schema := &genai.Schema{
	// 	Type: genai.TypeObject,
	// 	Properties: map[string]*genai.Schema{
	// 		"location": {
	// 			Type:        genai.TypeString,
	// 			Description: " San Francisco, CA or a zip code e.g. 95616",
	// 		},
	// 		"title": {
	// 			Type:        genai.TypeString,
	// 			Description: "Any movie title",
	// 		},
	// 	},
	// 	Required: []string{"location"},
	// }

	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"title": {
				Type:        genai.TypeString,
				Description: "summary user content to title",
			},
		},
		Required: []string{"title"},
	}

	movieTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "func_summary",
			Description: "summary content",
			Parameters:  schema,
		}},
	}

	model := client.GenerativeModel("gemini-1.5-pro-latest")

	// Before initiating a conversation, we tell the model which tools it has
	// at its disposal.
	model.Tools = []*genai.Tool{movieTool}
	model.ToolConfig = &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingAny,
		},
	}

	// model.SystemInstruction = genai.NewUserContent(genai.Text(gemini.PROMPT_PROCESS_CONTENT_CN))

	// For using tools, the chat mode is useful because it provides the required
	// chat context. A model needs to have tools supplied to it in the chat
	// history so it can use them in subsequent conversations.
	//
	// The flow of message expected here is:
	//
	// 1. We send a question to the model
	// 2. The model recognizes that it needs to use a tool to answer the question,
	//    an returns a FunctionCall response asking to use the tool.
	// 3. We send a FunctionResponse message, simulating the return value of
	//    the tool for the model's query.
	// 4. The model provides its text answer in response to this message.
	content := "Docker 支持 64 位版本 CentOS 7/8，并且要求内核版本不低于 3.10。 CentOS 7 满足最低内核的要求，但由于内核版本比较低，部分功能（如 overlay2 存储层驱动）无法使用，并且部分功能可能不太稳定。"
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	// defer cancel()
	// res, err := model.GenerateContent(ctx, genai.Text(content))
	// if err != nil {
	// 	t.Fatal(err)
	// }

	session := model.StartChat()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	res, err := session.SendMessage(ctx, genai.Text(content))
	if err != nil {
		log.Fatalf("session.SendMessage: %v", err)
	}

	part := res.Candidates[0].Content.Parts[0]
	funcall, ok := part.(genai.FunctionCall)
	if !ok || funcall.Name != "func_summary" {
		log.Fatalf("expected FunctionCall to find_theaters: %v", part)
	}

	// Expect the model to pass a proper string "location" argument to the tool.
	if _, ok := funcall.Args["title"].(string); !ok {
		log.Fatalf("expected string: %v", funcall.Args["location"])
	}

	t.Log(funcall)
}
