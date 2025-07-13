package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: "start",
	Short: "Start a container",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Start command called [placeholder]")
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
