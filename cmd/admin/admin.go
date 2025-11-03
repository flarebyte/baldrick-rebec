package admin

import (
    "github.com/flarebyte/baldrick-rebec/cmd/admin/conversation"
    configcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/config"
    dbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/db"
    scripcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/script"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/message"
    wscmd "github.com/flarebyte/baldrick-rebec/cmd/admin/workspace"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/role"
    prjcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/project"
    tagcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/tag"
    pkgcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/package"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/workflow"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/task"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/experiment"
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
    AdminCmd.AddCommand(scripcmd.ScriptCmd)
    AdminCmd.AddCommand(message.MessageCmd)
    AdminCmd.AddCommand(wscmd.WorkspaceCmd)
    AdminCmd.AddCommand(role.RoleCmd)
    AdminCmd.AddCommand(prjcmd.ProjectCmd)
    AdminCmd.AddCommand(tagcmd.TagCmd)
    AdminCmd.AddCommand(pkgcmd.PackageCmd)
    AdminCmd.AddCommand(workflow.WorkflowCmd)
    AdminCmd.AddCommand(task.TaskCmd)
    AdminCmd.AddCommand(experiment.ExperimentCmd)
    AdminCmd.AddCommand(srvcmd.ServerCmd)
}
