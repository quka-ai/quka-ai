package process

import (
	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/queue"
	"github.com/quka-ai/quka-ai/pkg/register"
)

func RSSQueue() *queue.RSSQueue {
	return p.rssQueue
}

func PodcastQueue() *queue.PodcastQueue {
	return p.podcastQueue
}

type Process struct {
	cron         *cron.Cron
	core         *core.Core
	asynqClient  *asynq.Client
	asynqServer  *asynq.Server
	asynqMux     *asynq.ServeMux
	rssQueue     *queue.RSSQueue
	podcastQueue *queue.PodcastQueue
}

var p *Process

type ProcessKey struct{}

func NewProcess(core *core.Core) *Process {
	p = &Process{
		cron: cron.New(),
		core: core,
	}

	// 创建共享的 asynq Client 和 Server（在注册处理器之前）
	cfg := core.Cfg().Redis

	// 创建 Client（用于入队任务）
	// 需要先创建 Redis 连接选项
	var redisOpt asynq.RedisConnOpt
	if cfg.Cluster {
		redisOpt = asynq.RedisClusterClientOpt{
			Addrs:    cfg.ClusterAddrs,
			Password: cfg.Password,
		}
	} else {
		redisOpt = asynq.RedisClientOpt{
			Network:  "tcp",
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}
	p.asynqClient = asynq.NewClient(redisOpt)

	// 创建 Server（用于消费任务），总并发数为 5（RSS: 3 + Podcast: 2）
	keyPrefix := cfg.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = "quka"
	}
	p.asynqServer = asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    5,
		StrictPriority: false,
		Queues: map[string]int{
			queue.RSSQueueName:     3, // RSS队列优先级
			queue.PodcastQueueName: 2, // Podcast队列优先级
		},
	})

	p.asynqMux = asynq.NewServeMux()

	for _, h := range register.ResolveFuncHandlers[*Process](ProcessKey{}) {
		h(p)
	}

	return p
}

func (p *Process) Cron() *cron.Cron {
	return p.cron
}

func (p *Process) Core() *core.Core {
	return p.core
}

func (p *Process) AsynqClient() *asynq.Client {
	return p.asynqClient
}

func (p *Process) AsynqServerMux() *asynq.ServeMux {
	return p.asynqMux
}

func (p *Process) SetAsynqClient(client *asynq.Client) {
	p.asynqClient = client
}

func (p *Process) SetAsynqServer(server *asynq.Server) {
	p.asynqServer = server
}

func (p *Process) Start() {
	StartKnowledgeProcess(p.core, 10)
	p.cron.Start()
	go p.asynqServer.Run(p.asynqMux)
}

func (p *Process) Stop() {
	// 停止 cron 调度器
	if p.cron != nil {
		ctx := p.cron.Stop()
		<-ctx.Done()
	}

	// 关闭 RSS Queue
	if p.rssQueue != nil {
		p.rssQueue.Shutdown()
	}

	// 关闭 Podcast Queue
	if p.podcastQueue != nil {
		p.podcastQueue.Shutdown()
	}
}

func (p *Process) SetRSSQueue(rssQueue *queue.RSSQueue) {
	p.rssQueue = rssQueue
}

func (p *Process) SetPodcastQueue(podcastQueue *queue.PodcastQueue) {
	p.podcastQueue = podcastQueue
}
