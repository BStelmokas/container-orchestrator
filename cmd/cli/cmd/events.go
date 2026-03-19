package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// eventsResponse mirrors GET /api/events.
type eventsResponse struct {
	Events []struct {
		Time string `json:"time"`
		Category string `json:"category"`
		Message string `json:"message"`
	} `json:"events"`
}

var eventsCmd = &cobra.Command{
	Use: "events",
	Short: "Show recent control-plane events",
	RunE: func(cmd *cobra.Command, args []string) error {
		var resp eventsResponse
		if err := doJSONRequest("GET", "/api/events", nil, &resp); err != nil {
			return err
		}

		if len(resp.Events) == 0 {
			fmt.Println("No events found.")
			return nil
		}

		for _, event := range resp.Events {
			fmt.Printf("[%s] %s %s\n", event.Time, event.Category, event.Message)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
}
