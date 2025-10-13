package admin

import (
	"github.com/flarebyte/baldrick-rebec/cmd/admin/conversation"
	"github.com/spf13/cobra"
)

var AdminCmd = &cobra.Command{
	Use:   "admin",
	Short: "TODO: Describe the 'admin' command",
}

func init() {
	AdminCmd.AddCommand(conversation.ConversationCmd)
}
