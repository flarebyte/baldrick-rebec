package queue

import "github.com/spf13/cobra"

var QueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage work queues",
}
