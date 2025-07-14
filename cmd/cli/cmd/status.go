package cmd

import (
	"fmt"
	"log"

	"orchestrator/internal/manager"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [container_id_or_name]",
	Short: "Status of a specific container by ID or name",
	Args:  cobra.ExactArgs(1), // Ensures one argument is passed
	Run: func(cmd *cobra.Command, args []string) {
		containerID := args[0]

		mgr, err := manager.NewContainerManager()
		if err != nil {
			log.Fatalf("Failed to create container manager: %v", err)
		}

		containers, err := mgr.ListContainers()
		if err != nil {
			log.Fatalf("Failed to list containers: %v", err)
		}

		// Search for the container by ID or name
		for _, c := range containers {
			if c.ID == containerID || (len(c.Names) > 0 && c.Names[0] == "/"+containerID) {
				fmt.Printf("Container found:\nID: %s\nImage: %s\nStatus: %s\n", c.ID[:12], c.Image, c.Status)
				return
			}
		}

		fmt.Printf("Container %q not found.\n", containerID)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
