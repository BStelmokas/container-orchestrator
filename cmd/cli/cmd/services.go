package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// servicesResponse mirrors the canonical GET /api/services response shape.
type serviceResponse struct {
	Services []struct {
		Name              string `json:"name"`
		Image             string `json:"image"`
		DesiredReplicas   int    `json:"desiredReplicas"`
		RunningReplicas   int    `json:"runningReplicas"`
		HealthyReplicas   int    `json:"healthyReplicas"`
		UnhealthyReplicas int    `json:"unhealthyReplicas"`
		UnknownReplicas   int    `json:"unknownReplicas"`
		OutdatedReplicas  int    `json:"outdatedReplicas"`
		Generation        int64  `json:"generation"`
		OverallStatus     string `json:"overallSatus"`
	} `json:"services"`
}

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List all services",
	RunE: func(cmd *cobra.Command, args []string) error {
		var resp serviceResponse
		if err := doJSONRequest("GET", "/api/services", nil, &resp); err != nil {
			return err
		}

		if len(resp.Services) == 0 {
			fmt.Println("No services found.")
			return nil
		}

		for _, svc := range resp.Services {
			fmt.Printf(
				"%s image=%s desired=%d running=%d healthy=%d unhealthy=%d unknown=%d outdated=%d generation=%d status=%s\n",
				svc.Name,
				svc.Image,
				svc.DesiredReplicas,
				svc.RunningReplicas,
				svc.HealthyReplicas,
				svc.UnhealthyReplicas,
				svc.UnknownReplicas,
				svc.OutdatedReplicas,
				svc.Generation,
				svc.OverallStatus,
			)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(servicesCmd)
}
