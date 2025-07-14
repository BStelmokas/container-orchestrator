package cmd

import (
	"fmt"
	"log"

	"orchestrator/internal/manager"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use: "list",
	Short: "List all containers (running and stopped)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("List containers...")

		mgr, err := manager.NewContainerManager()
		if err != nil {
			log.Fatalf("Failed to create container manager: %v", err)
		}

		containers, err := mgr.ListContainers()
		if err != nil {
			log.Fatalf("Failed to list containers: %v", err)
		}

		if len(containers) == 0 {
			fmt.Println("No containers found.")
			return
		}

		for _, c := range containers {
			fmt.Printf("Container ID: %s, Image: %s, Status: %s\n", c.ID[:12], c.Image, c.Status)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
