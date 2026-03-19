package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use: "restart SERVICE_NAME",
	Short: "Request a rolling restart for a service",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		var resp map[string]any
		if err := doJSONRequest("POST", "/api/services/"+serviceName+"/restart", nil, &resp); err != nil {
			return err
		}

		fmt.Printf("Requested rolling restart for service %q\n", serviceName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
