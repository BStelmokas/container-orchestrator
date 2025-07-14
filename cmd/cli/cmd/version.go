package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use: "version",
	Short: "Show the orchestrator CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("orechestrator version 0.1.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
