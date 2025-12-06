package admin

import (
	bbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/blackboard"
	configcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/config"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/conversation"
	dbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/db"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/experiment"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/message"
	pkgcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/package"
	prjcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/project"
	qcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/queue"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/role"
	scripcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/script"
	srvcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/server"
	snapcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/snapshot"
	stickcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/stickie"
	stickrelcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/stickie_rel"
	storecmd "github.com/flarebyte/baldrick-rebec/cmd/admin/store"
	tagcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/tag"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/task"
	tccmd "github.com/flarebyte/baldrick-rebec/cmd/admin/testcase"
	toolcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/tool"
	topiccmd "github.com/flarebyte/baldrick-rebec/cmd/admin/topic"
	vaultcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/vault"
	promptcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/prompt"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/workflow"
	wscmd "github.com/flarebyte/baldrick-rebec/cmd/admin/workspace"
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
	AdminCmd.AddCommand(stickrelcmd.StickieRelCmd)
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
	AdminCmd.AddCommand(snapcmd.SnapshotCmd)
	AdminCmd.AddCommand(vaultcmd.VaultCmd)
	AdminCmd.AddCommand(toolcmd.ToolCmd)
	AdminCmd.AddCommand(promptcmd.PromptCmd)
}
