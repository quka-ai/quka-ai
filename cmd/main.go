package main

import (
	"fmt"
	"os"

	"github.com/quka-ai/quka-ai/cmd/service"
	"github.com/spf13/cobra"
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
