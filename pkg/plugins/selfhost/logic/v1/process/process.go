package process

import (
	"log/slog"

	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/register"
	"github.com/samber/lo"

	commsrv "github.com/quka-ai/quka-ai/pkg/plugins/selfhost/srv"
)

func findToolInConfig(tools []string, find string) bool {
	_, exist := lo.Find(tools, func(tool string) bool {
		return tool == find
	})
	return exist
}

func SetupProcess(core *commsrv.PluginCore) {

	chunkTaskProcess := NewContentTaskProcess(core.AppCore, core.Cfg.ChunkService)

	slog.Info("Register new process", slog.String("name", "content_chunk_task"))
	register.RegisterFunc(process.ProcessKey{}, func(provider *process.Process) {
		provider.Cron().AddFunc("*/1 * * * *", func() {
			chunkTaskProcess.ProcessTasks()
		})
	})
}
