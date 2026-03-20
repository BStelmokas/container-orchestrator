package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// deployRequest matches the canonical POST /api/services payload.
type deployRequest struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Replicas int    `json:"replicas"`
}

var (
	deployName     string
	deployImage    string
	deployReplicas int
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Create or update a service",
	RunE: func(cmd *cobra.Command, args []string) error {
		req := deployRequest{
			Name:     deployName,
			Image:    deployImage,
			Replicas: deployReplicas,
		}

		var resp map[string]any
		if err := doJSONRequest("POST", "/api/services", req, &resp); err != nil {
			return err
		}

		fmt.Printf("Stored service %q with image %q and %d replicas\n", deployName, deployImage, deployReplicas)
		return nil
	},
}

func init() {
	deployCmd.Flags().StringVar(&deployName, "name", "", "Logical service name (required)")
	deployCmd.Flags().StringVar(&deployImage, "image", "", "Docker image to deploy (required)")
	deployCmd.Flags().IntVar(&deployReplicas, "replicas", 1, "Desired number of replicas")

	_ = deployCmd.MarkFlagRequired("name")
	_ = deployCmd.MarkFlagRequired("image")

	rootCmd.AddCommand(deployCmd)
}
