package cmd

import (
	"fmt"
	"log"

	"orchestrator/internal/manager"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use: "stop [container_id_or_name]",
	Short: "Stop a running container by ID or name",
	Args: cobra.ExactArgs(1), // Requires exactly one argument
	Run: func(cmd *cobra.Command, args []string) {
		containerID := args[0]

		// Create Docker client
		mgr, err := manager.NewContainerManager()
		if err != nil {
			log.Fatalf("Failed to create container manager: %v", err)
		}

		// Attempt to stop the container
		err = mgr.StopContainer(containerID)
		if err != nil {
			log.Fatalf("Failed to stop container: %v", err)
		}

		fmt.Printf("Succesfully stopped container: %s\n", containerID)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
