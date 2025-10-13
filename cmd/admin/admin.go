package admin

import (
	"github.com/spf13/cobra"
	"github.com/your-org/your-cli/cmd/admin/conversation"
)

var AdminCmd = &cobra.Command{
	Use:   "admin",
	Short: "TODO: Describe the 'admin' command",
}

func init() {
	AdminCmd.AddCommand(conversation.ConversationCmd)
}
