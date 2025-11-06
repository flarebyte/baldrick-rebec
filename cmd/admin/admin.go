package admin

import (
    "github.com/flarebyte/baldrick-rebec/cmd/admin/conversation"
    configcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/config"
    dbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/db"
    scripcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/script"
    qcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/queue"
    tccmd "github.com/flarebyte/baldrick-rebec/cmd/admin/testcase"
    storecmd "github.com/flarebyte/baldrick-rebec/cmd/admin/store"
    bbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/blackboard"
    stickcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/stickie"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/message"
    wscmd "github.com/flarebyte/baldrick-rebec/cmd/admin/workspace"
    "github.com/flarebyte/baldrick-rebec/cmd/admin/role"
    prjcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/project"
    tagcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/tag"
    topiccmd "github.com/flarebyte/baldrick-rebec/cmd/admin/topic"
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
    AdminCmd.AddCommand(qcmd.QueueCmd)
    AdminCmd.AddCommand(tccmd.TestcaseCmd)
    AdminCmd.AddCommand(storecmd.StoreCmd)
    AdminCmd.AddCommand(bbcmd.BlackboardCmd)
    AdminCmd.AddCommand(stickcmd.StickieCmd)
    AdminCmd.AddCommand(scripcmd.ScriptCmd)
    AdminCmd.AddCommand(message.MessageCmd)
    AdminCmd.AddCommand(wscmd.WorkspaceCmd)
    AdminCmd.AddCommand(role.RoleCmd)
    AdminCmd.AddCommand(prjcmd.ProjectCmd)
    AdminCmd.AddCommand(tagcmd.TagCmd)
    AdminCmd.AddCommand(topiccmd.TopicCmd)
    AdminCmd.AddCommand(pkgcmd.PackageCmd)
    AdminCmd.AddCommand(workflow.WorkflowCmd)
    AdminCmd.AddCommand(task.TaskCmd)
    AdminCmd.AddCommand(experiment.ExperimentCmd)
    AdminCmd.AddCommand(srvcmd.ServerCmd)
}
