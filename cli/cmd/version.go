package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version information for the CodeRunr CLI.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("CodeRunr CLI v1.0.0")
			fmt.Println("Compatible with CodeRunr API v2")
			fmt.Println("Built with Go and Cobra framework")
		},
	}

	return cmd
}
