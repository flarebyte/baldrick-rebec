package topic

import "github.com/spf13/cobra"

var TopicCmd = &cobra.Command{
	Use:   "topic",
	Short: "Manage topics (role-scoped)",
}
