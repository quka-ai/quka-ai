package core

import (
	"github.com/quka-ai/quka-ai/app/core/srv"
	centrifugeManager "github.com/quka-ai/quka-ai/pkg/socket/centrifuge"
)

func SetupSrv(core *Core) {
	// 初始化时加载AI配置
	aiApplyFunc := core.loadInitialAIConfig()

	// 创建Centrifuge设置函数
	centrifugeSetupFunc := func() (srv.CentrifugeManager, error) {
		// 从配置文件创建Centrifuge配置
		config := &centrifugeManager.Config{
			MaxConnections:    core.cfg.Centrifuge.MaxConnections,
			HeartbeatInterval: core.cfg.Centrifuge.HeartbeatInterval,
			DeploymentMode:    core.cfg.Centrifuge.DeploymentMode,
			RedisURL:          core.cfg.Centrifuge.RedisURL,
			RedisCluster:      core.cfg.Centrifuge.RedisCluster,
			EnablePresence:    core.cfg.Centrifuge.EnablePresence,
			EnableHistory:     core.cfg.Centrifuge.EnableHistory,
			EnableRecovery:    core.cfg.Centrifuge.EnableRecovery,
			AllowedOrigins:    core.cfg.Centrifuge.AllowedOrigins,
			MaxChannelLength:  core.cfg.Centrifuge.MaxChannelLength,
			MaxMessageSize:    core.cfg.Centrifuge.MaxMessageSize,
		}
		manager, err := centrifugeManager.NewManager(config, core.Store())
		if err != nil {
			return nil, err
		}
		// 返回接口类型
		return manager, nil
	}

	core.srv = srv.SetupSrvs(
		aiApplyFunc, // ai provider select
		// centrifuge websocket (支持分布式)
		srv.ApplyCentrifuge(centrifugeSetupFunc),
	)
}
