package ollama_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/ollama"
	"github.com/quka-ai/quka-ai/pkg/utils"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

func new() *ollama.Driver {
	fmt.Println(os.Getenv("BREW_API_AI_OLLAMA_TOKEN"), os.Getenv("BREW_API_AI_OLLAMA_ENDPOINT"))
	return ollama.New(os.Getenv("BREW_API_AI_OLLAMA_TOKEN"), os.Getenv("BREW_API_AI_OLLAMA_ENDPOINT"), ai.ModelName{
		EmbeddingModel: "bge-m3",
	})
}

func Test_Embedding(t *testing.T) {
	d := new()

	text1 := ""
	text2 := ""

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	res, err := d.EmbeddingForDocument(ctx, "test", []string{text1, text2})
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, len(res.Data), 0)

	a := lo.Map(res.Data[0], func(item float32, _ int) float64 {
		return float64(item)
	})
	b := lo.Map(res.Data[1], func(item float32, _ int) float64 {
		return float64(item)
	})

	t.Log(utils.Cosine(a, b))
}

func TestReRank(t *testing.T) {
	query := ""

	texts := []string{""}
	d := new()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := d.EmbeddingForDocument(ctx, "test", texts)
	if err != nil {
		t.Fatal(err)
	}

	queryResult, err := d.EmbeddingForDocument(ctx, "test", []string{query})
	if err != nil {
		t.Fatal(err)
	}

	a := lo.Map(queryResult.Data[0], func(item float32, _ int) float64 {
		return float64(item)
	})

	for i, v := range res.Data {
		b := lo.Map(v, func(item float32, _ int) float64 {
			return float64(item)
		})

		t.Log(i, utils.Cosine(a, b))
	}
}
