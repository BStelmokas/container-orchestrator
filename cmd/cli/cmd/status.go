package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use: "status",
	Short: "Status of the containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Status command called [placeholder]")
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
