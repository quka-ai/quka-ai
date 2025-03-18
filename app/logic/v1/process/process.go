package process

import (
	"github.com/robfig/cron/v3"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/pkg/register"
)

type Process struct {
	cron *cron.Cron
	core *core.Core
}

var p *Process

type ProcessKey struct{}

func NewProcess(core *core.Core) *Process {
	p = &Process{
		cron: cron.New(),
		core: core,
	}

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

func (p *Process) Start() {
	StartKnowledgeProcess(p.core, 10)
	p.cron.Start()
}
