package main

import (
    "fmt"
    "log"

    "orchestrator/internal/manager"


)

func main() {
	manager, err := manager.NewContainerManager()
	if err != nil {
		log.Fatalf("Error creating container manager: %v", err)
	}

	// Start a container
	image := "nginx:latest"
	name := "my-nginx"
	fmt.Println("Starting container...")
	containerID, err := manager.StartContainer(image, name)
	if err != nil {
		log.Fatalf("Start error: %v", err)
	}
	fmt.Printf("Started container with ID: %s\n", containerID[:12])

	// Start monitoring container health
	healthURL := "http://localhost:80"
	manager.StartHealthMonitor(containerID, name, image, healthURL)

	// List containers to verify running state
	fmt.Println("Listing containers...")
	containers, err := manager.ListContainers()
	if err != nil {
		log.Fatalf("List error: %v", err)
	}
	for _, c := range containers {
		fmt.Printf("Container ID: %s, Image: %s, Status: %s\n", c.ID[:12], c.Image, c.Status)
	}

	// Keep main running forever so the health monitor goroutine doesn't exit
	select {}
}
