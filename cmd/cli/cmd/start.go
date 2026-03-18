package cmd

import (
	"fmt"
	"log"

	"orchestrator/internal/manager"

	"github.com/spf13/cobra"
)

var (
	imageName     string
	containerName string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a container",
	Run: func(cmd *cobra.Command, args []string) {
		mgr, err := manager.NewContainerManager()
		if err != nil {
			log.Fatalf("Error creating manager: %v", err)
		}

		fmt.Println("Starting container...")

		id, err := mgr.StartContainer(containerName, imageName, containerName, 1)
		if err != nil {
			log.Fatalf("Failed to start container: %v", err)
		}

		fmt.Printf("Started container with ID: %s\n", id)
	},
}

func init() {
	startCmd.Flags().StringVar(&imageName, "image", "", "Docker image to use (required)")
	startCmd.Flags().StringVar(&containerName, "name", "", "Name for the container (required)")

	startCmd.MarkFlagRequired("image")
	startCmd.MarkFlagRequired("name")

	rootCmd.AddCommand(startCmd)
}
