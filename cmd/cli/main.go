package main

import (
	"log"

	"orchestrator/cmd/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalf("CLI error: %v", err)
	}
}
