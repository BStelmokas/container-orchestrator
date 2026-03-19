package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var serverURL string

var rootCmd = &cobra.Command{
	Use:   "orchestratorctl",
	Short: "Control-plane CLI for the container orchestrator",
	Long:  "A service-oriented CLI that talks to the orchestrator API to deploy, scale, restart, inspect, and delete services.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Add a single canonical server flag so every command can target the orchestrator API.
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "Base URL of the orchestrator API server")
}
