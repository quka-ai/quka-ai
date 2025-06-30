package deepseek_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/quka-ai/quka-ai/pkg/ai"
	openai "github.com/quka-ai/quka-ai/pkg/ai/deepseek"
	"github.com/quka-ai/quka-ai/pkg/types"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

func new() *openai.Driver {
	return openai.New(os.Getenv("BREW_API_AI_DEEPSEEK_TOKEN"), os.Getenv("BREW_API_AI_DEEPSEEK_ENDPOINT"), ai.ModelName{
		ChatModel: os.Getenv("BREW_API_AI_DEEPSEEK_CHAT_MODEL"),
	})
}

func Test_Generate(t *testing.T) {
	d := new()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	opts := d.NewQuery(ctx, "deepseek-chat-v3", []*types.MessageContext{
		{
			Role:    types.USER_ROLE_USER,
			Content: "我的车现在停在哪里？",
		},
	})
	prompt := ai.BuildRAGPrompt(`
		以下是关于回答用户提问的“参考内容”，这些内容都是历史记录，其中提到的时间点无法与当前时间进行参照：
		--------------------------------------
		${relevant_passage}
		--------------------------------------
		你需要结合“参考内容”来回答用户的提问，
		注意，“参考内容”中可能有部分内容描述的是同一件事情，但是发生的时间不同，当你无法选择应该参考哪一天的内容时，可以结合用户提出的问题进行分析。
		如果你从上述内容中找到了用户想要的答案，可以结合内容相关的属性来给到用户更多的帮助，比如参考“事件发生时间”来告诉用户这件事发生在哪天。
		请你使用 ${lang} 语言，以Markdown格式回复用户。
	`, ai.NewDocs([]*types.PassageInfo{
		{
			ID:       "xcjoijoijo12",
			Content:  "我有一辆白色的车",
			DateTime: "2024-06-03 15:20:10",
		},
		{
			ID:       "xcjoiaajoijo12",
			Content:  "我有一辆白色的自行车",
			DateTime: "2024-06-03 15:20:10",
		},
		{
			ID:       "3333oij1111oijo12",
			Content:  "周五我把车停在了B3层",
			DateTime: "2024-09-03 15:20:10",
		},
		{
			ID:       "xcjoij12312ijo12",
			Content:  "我昨天把车停在了B2层",
			DateTime: "2024-09-20 15:20:10",
		},
		{
			ID:       "3333oijoijo12",
			Content:  "停车楼里有十辆车",
			DateTime: "2024-06-03 15:20:10",
		},
	}), d)

	opts.WithPrompt(prompt)
	res, err := opts.Query()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(res)
}

func Test_Summarize(t *testing.T) {
	content := `
通过docker部署向量数据库postgres，pgvector的docker部署方式：
docker run --restart=always \
-id \
--name=postgresql \
-v postgre-data:/var/lib/postgresql/data \
-p 5432:5432 \
-e POSTGRES_PASSWORD=123456 \
-e LANG=C.UTF-8 \
-e POSTGRES_USER=root \
pgvector/pgvector:pg16
	`

	d := new()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	resp, err := d.Summarize(ctx, &content)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp)
}

func Test_EnhanceQuery(t *testing.T) {
	query := "喝小红有什么作用？"

	d := new()
	opts := ai.NewEnhance(context.Background(), d)
	opts.WithPrompt(ai.PROMPT_ENHANCE_QUERY_CN)
	res, err := opts.EnhanceQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(res, res.Usage)
}
