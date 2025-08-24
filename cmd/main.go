package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/quka-ai/quka-ai/cmd/service"
	_ "github.com/quka-ai/quka-ai/pkg/plugins/selfhost"
)

func main() {
	root := &cobra.Command{
		Use:   "service",
		Short: "service",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("empty command")
		},
	}

	root.AddCommand(service.NewCommand(), service.NewProcessCommand())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
