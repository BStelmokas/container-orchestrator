package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use: "stop",
	Short: "Stop a container",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Stop container called [placeholder]")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
