package srv

import (
	"fmt"

	"github.com/quka-ai/quka-ai/app/core"
)

func NewPluginCore(c *core.Core) (*PluginCore, error) {
	customConfig := core.NewCustomConfigPayload[CustomConfig]()
	if err := c.Cfg().LoadCustomConfig(&customConfig); err != nil {
		return nil, fmt.Errorf("Failed to install custom config, %w", err)
	}

	pluginCore := &PluginCore{
		Cfg:     customConfig.CustomConfig,
		AppCore: c,
		Srv:     &Srv{},
	}

	return pluginCore, nil
}

type PluginCore struct {
	Cfg     CustomConfig
	AppCore *core.Core
	Srv     *Srv
}

type Srv struct {
}
