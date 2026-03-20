package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// scaleRequest matches the canonical POST /api/services/:name/scale payload.
type scaleRequest struct {
	Replicas int `json:"replicas"`
}

var scaleReplicas int

var scaleCmd = &cobra.Command{
	Use:   "scale SERVICE_NAME",
	Short: "Set a service to an exact replica count",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		req := scaleRequest{
			Replicas: scaleReplicas,
		}

		var resp map[string]any
		if err := doJSONRequest("POST", "/api/services/"+serviceName+"/scale", req, &resp); err != nil {
			return err
		}

		fmt.Printf("Scaled service %q to %d replicas\n", serviceName, scaleReplicas)
		return nil
	},
}

func init() {
	scaleCmd.Flags().IntVar(&scaleReplicas, "replicas", 0, "Exact desired replica count (required)")
	_ = scaleCmd.MarkFlagRequired("replicas")

	rootCmd.AddCommand(scaleCmd)
}
