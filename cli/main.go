package main

import (
	"fmt"
	"os"

	"github.com/coderunr/cli/cmd"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "coderunr",
		Short:   "CodeRunr CLI - Execute code in various programming languages",
		Long:    `A command line interface for CodeRunr code execution engine.`,
		Version: fmt.Sprintf("%s (%s) built at %s", version, commit, date),
	}

	// Global flags
	rootCmd.PersistentFlags().StringP("url", "u", "http://localhost:2000", "CodeRunr API URL")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().String("output", "auto", "Output format (auto, json, plain)")

	// Add subcommands
	rootCmd.AddCommand(
		cmd.NewExecuteCommand(),
		cmd.NewPackageCommand(),
		cmd.NewListCommand(),
		cmd.NewVersionCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
