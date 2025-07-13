package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use: "list",
	Short: "List the containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("List command called [placeholder]")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
