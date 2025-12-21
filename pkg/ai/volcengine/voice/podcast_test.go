package voice

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/quka-ai/quka-ai/pkg/utils"
)

func TestGenPodcast(t *testing.T) {
	p := NewPodCaster(os.Getenv("QUKA_TEST_PODCAST_APPID"), os.Getenv("QUKA_TEST_PODCAST_ACCESS_TOKEN"))

	text := `
但串联起来看，这些事件共同指向一个方向：AI 正在从"大力出奇迹"转向"巧劲见真章"。规模化不再是唯一的道路，架构创新、专业化、安全性、可解释性——这些曾经被忽视的方面正在变得越来越重要。
对于开发者来说，这是个好消息。工具变得更强大、更易用、更便宜；小模型也能办大事；边缘计算和隐私保护成为可能。AI 不再是高高在上的云端服务，而是可以真正融入日常工作流程的伙伴。
	`

	// 进度回调函数
	progressCallback := func() {
		t.Log("Progress updated")
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
	result, err := p.Gen(ctx, utils.RandomStr(10), text, true, false, progressCallback)
	if err != nil {
		t.Fatal(err)
	}

	raw, _ := json.Marshal(result)
	t.Log(string(raw))
}
