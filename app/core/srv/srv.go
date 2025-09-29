package srv

import (
	"github.com/quka-ai/quka-ai/pkg/socket/firetower"
	"github.com/quka-ai/quka-ai/pkg/types"
)

type Srv struct {
	rbac       *RBACSrv
	ai         *AI
	tower      *Tower
	centrifuge CentrifugeManager
}

func SetupSrvs(opts ...ApplyFunc) *Srv {
	a := &Srv{
		rbac: SetupRBACSrv(), // 角色鉴权
	}

	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (s *Srv) RBAC() *RBACSrv {
	return s.rbac
}

func (s *Srv) AI() AIDriver {
	return s.ai
}

func (t *Tower) Pusher() *firetower.SelfPusher[PublishData] {
	return t.pusher
}

func (s *Srv) Tower() *Tower {
	return s.tower
}

func (s *Srv) Centrifuge() CentrifugeManager {
	return s.centrifuge
}

// ReloadAI 重新加载AI配置
func (s *Srv) ReloadAI(models []types.ModelConfig, modelProviders []types.ModelProvider, usage Usage) error {
	news, err := SetupAI(models, modelProviders, usage)
	if err != nil {
		return err
	}
	s.ai = news
	return nil
}

// GetAIStatus 获取AI系统状态
func (s *Srv) GetAIStatus() map[string]interface{} {
	if s.ai == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	return map[string]interface{}{
		"status":            "running",
		"chat_available":    s.ai.chatDefault != nil,
		"embed_available":   s.ai.embedDefault != nil,
		"vision_available":  s.ai.visionDefault != nil,
		"rerank_available":  s.ai.rerankDefault != nil,
		"reader_available":  s.ai.readerDefault != nil,
		"enhance_available": s.ai.enhanceDefault != nil,
	}
}
