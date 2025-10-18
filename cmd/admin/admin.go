package admin

import (
    "github.com/flarebyte/baldrick-rebec/cmd/admin/conversation"
    configcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/config"
    dbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/db"
    oscmd "github.com/flarebyte/baldrick-rebec/cmd/admin/os"
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
    AdminCmd.AddCommand(configcmd.ConfigCmd)
    AdminCmd.AddCommand(dbcmd.DBCmd)
    AdminCmd.AddCommand(oscmd.OSCmd)
    AdminCmd.AddCommand(message.MessageCmd)
    AdminCmd.AddCommand(srvcmd.ServerCmd)
}
