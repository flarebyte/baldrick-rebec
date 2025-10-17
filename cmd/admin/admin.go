package admin

import (
    "github.com/flarebyte/baldrick-rebec/cmd/admin/conversation"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/message"
    srvcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/server"
    "github.com/spf13/cobra"
)

var AdminCmd = &cobra.Command{
    Use:   "admin",
    Short: "TODO: Describe the 'admin' command",
}

func init() {
    AdminCmd.AddCommand(conversation.ConversationCmd)
    AdminCmd.AddCommand(message.MessageCmd)
    AdminCmd.AddCommand(srvcmd.ServerCmd)
}
