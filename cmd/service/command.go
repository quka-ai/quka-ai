package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/quka-ai/quka-ai/app/core"
	"github.com/quka-ai/quka-ai/app/logic/v1/process"
	"github.com/quka-ai/quka-ai/pkg/plugins"
)

type Options struct {
	ConfigPath string
	Init       string
}

func (o *Options) AddFlags(flagSet *pflag.FlagSet) {
	// Add flags for generic options
	flagSet.StringVarP(&o.ConfigPath, "config", "c", "", "init api by given config")
	flagSet.StringVarP(&o.Init, "init", "i", "selfhost", "start service after initialize")
}

func NewCommand() *cobra.Command {
	opts := &Options{}
	cmd := &cobra.Command{
		Use:   "service",
		Short: "chat service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(opts)
		},
	}
	opts.AddFlags(cmd.Flags())
	return cmd
}

func Run(opts *Options) error {
	app := core.MustSetupCore(core.MustLoadBaseConfig(opts.ConfigPath))
	plugins.Setup(app.InstallPlugins, opts.Init)
	process.NewProcess(app).Start()
	serve(app)

	return nil
}

func NewProcessCommand() *cobra.Command {
	opts := &Options{}
	cmd := &cobra.Command{
		Use:   "process",
		Short: "process",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunProcess(opts)
		},
	}
	opts.AddFlags(cmd.Flags())
	return cmd
}

func RunProcess(opts *Options) error {
	app := core.MustSetupCore(core.MustLoadBaseConfig(opts.ConfigPath))
	plugins.Setup(app.InstallPlugins, opts.Init)
	process.NewProcess(app).Start()
	fmt.Println("Process starting...")
	sigs := make(chan os.Signal, 1)
	// 监听 os.Interrupt (Ctrl+C) 和 syscall.SIGTERM (kill)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	// 阻塞等待信号
	<-sigs
	return nil
}
