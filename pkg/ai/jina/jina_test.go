package jina_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/quka-ai/quka-ai/pkg/ai"
	"github.com/quka-ai/quka-ai/pkg/ai/jina"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

func new() *jina.Driver {
	return jina.New(os.Getenv("BREW_API_AI_JINA_TOKEN"), map[string]string{
		"rerank": "jina-reranker-v2-base-multilingual",
	})
}

func Test_Reader(t *testing.T) {
	d := new()

	resp, err := d.Reader(context.Background(), "https://brew.re")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(*resp)
}

func TestRerank(t *testing.T) {
	d := new()

	query := ""
	docs := []*ai.RerankDoc{{
		ID:      "1",
		Content: "Organic skincare for sensitive skin with aloe vera and chamomile: Imagine the soothing embrace of nature with our organic skincare range, crafted specifically for sensitive skin. Infused with the calming properties of aloe vera and chamomile, each product provides gentle nourishment and protection. Say goodbye to irritation and hello to a glowing, healthy complexion.",
	},
		{
			ID:      "2",
			Content: "Bio-Hautpflege für empfindliche Haut mit Aloe Vera und Kamille: Erleben Sie die wohltuende Wirkung unserer Bio-Hautpflege, speziell für empfindliche Haut entwickelt. Mit den beruhigenden Eigenschaften von Aloe Vera und Kamille pflegen und schützen unsere Produkte Ihre Haut auf natürliche Weise. Verabschieden Sie sich von Hautirritationen und genießen Sie einen strahlenden Teint.",
		}, {
			ID:      "3",
			Content: "New makeup trends focus on bold colors and innovative techniques: Step into the world of cutting-edge beauty with this seasons makeup trends. Bold, vibrant colors and groundbreaking techniques are redefining the art of makeup. From neon eyeliners to holographic highlighters, unleash your creativity and make a statement with every look.",
		}, {
			ID:      "4",
			Content: "Neue Make-up-Trends setzen auf kräftige Farben und innovative Techniken: Tauchen Sie ein in die Welt der modernen Schönheit mit den neuesten Make-up-Trends. Kräftige, lebendige Farben und innovative Techniken setzen neue Maßstäbe. Von auffälligen Eyelinern bis hin zu holografischen Highlightern – lassen Sie Ihrer Kreativität freien Lauf und setzen Sie jedes Mal ein Statement.",
		}, {
			ID:      "5",
			Content: "Cuidado de la piel orgánico para piel sensible con aloe vera y manzanilla: Descubre el poder de la naturaleza con nuestra línea de cuidado de la piel orgánico, diseñada especialmente para pieles sensibles. Enriquecidos con aloe vera y manzanilla, estos productos ofrecen una hidratación y protección suave. Despídete de las irritaciones y saluda a una piel radiante y saludable.",
		}, {
			ID:      "6",
			Content: "Las nuevas tendencias de maquillaje se centran en colores vivos y técnicas innovadoras: Entra en el fascinante mundo del maquillaje con las tendencias más actuales. Colores vivos y técnicas innovadoras están revolucionando el arte del maquillaje. Desde delineadores neón hasta iluminadores holográficos, desata tu creatividad y destaca en cada look.",
		}, {
			ID:      "7",
			Content: "针对敏感肌专门设计的天然有机护肤产品：体验由芦荟和洋甘菊提取物带来的自然呵护。我们的护肤产品特别为敏感肌设计，温和滋润，保护您的肌肤不受刺激。让您的肌肤告别不适，迎来健康光彩。",
		}, {
			ID:      "8",
			Content: "新的化妆趋势注重鲜艳的颜色和创新的技巧：进入化妆艺术的新纪元，本季的化妆趋势以大胆的颜色和创新的技巧为主。无论是霓虹眼线还是全息高光，每一款妆容都能让您脱颖而出，展现独特魅力。",
		}, {
			ID:      "9",
			Content: "敏感肌のために特別に設計された天然有機スキンケア製品: アロエベラとカモミールのやさしい力で、自然の抱擁を感じてください。敏感肌用に特別に設計された私たちのスキンケア製品は、肌に優しく栄養を与え、保護します。肌トラブルにさようなら、輝く健康な肌にこんにちは。",
		}, {
			ID:      "10",
			Content: "新しいメイクのトレンドは鮮やかな色と革新的な技術に焦点を当てています: 今シーズンのメイクアップトレンドは、大胆な色彩と革新的な技術に注目しています。ネオンアイライナーからホログラフィックハイライターまで、クリエイティビティを解き放ち、毎回ユニークなルックを演出しましょう。",
		}}
	res, usage, err := d.Rerank(context.Background(), query, docs)
	if err != nil {
		t.Fatal(err)
	}

	if usage != nil {
		t.Log("usage", usage.Usage.PromptTokens)
	}

	t.Log(res)
}
