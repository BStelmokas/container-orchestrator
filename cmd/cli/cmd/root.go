package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "cli",
	Short: "A CLI to manage Docker containers: start, stop, loop, list, and status",
}

// Adds all subcommands and runs the root
func Execute() error {
	return rootCmd.Execute()
}
