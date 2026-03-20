package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use: "get SERVICE_NAME",
	Short: "Get one service with full status",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		var resp map[string]any
		if err := doJSONRequest("GET", "/api/services/"+serviceName, nil, &resp); err != nil {
			return err
		}

		pretty, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(pretty))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
