package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use: "delete SERVICE_NAME",
	Short: "Delete a service and stop its running replicas",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		if err := doJSONRequest("DELETE", "/api/services/"+serviceName, nil, nil); err != nil {
			return err
		}

		fmt.Printf("Deleted service %q\n", serviceName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
