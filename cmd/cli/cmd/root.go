package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the base command that acts as the entry point to the CLI
var rootCmd = &cobra.Command{
	Use: "containercli",
	Short: "A simple CLI for managing Docker containers",
	Long: `containercli is a lightweight command-line tool for starting, stopping, listing, and monitoring Docker containers.`,
}

// Runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
