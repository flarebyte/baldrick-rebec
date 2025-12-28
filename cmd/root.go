package cmd

import (
	bbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/blackboard"
	configcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/config"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/conversation"
	dbcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/db"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/experiment"
	"github.com/flarebyte/baldrick-rebec/cmd/admin/message"
	pkgcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/package"
	prjcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/project"
	promptcmd "github.com/flarebyte/baldrick-rebec/cmd/admin/prompt"
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
	"github.com/flarebyte/baldrick-rebec/cmd/admin/workflow"
	wscmd "github.com/flarebyte/baldrick-rebec/cmd/admin/workspace"
	"github.com/flarebyte/baldrick-rebec/cmd/test"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rbc",
	Short: "TODO: Short description of your CLI",
	Long:  "TODO: Long description of your CLI tool.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(test.TestCmd)
	// Expose former `admin` subcommands at the root level
	rootCmd.AddCommand(conversation.ConversationCmd)
	rootCmd.AddCommand(configcmd.ConfigCmd)
	rootCmd.AddCommand(dbcmd.DBCmd)
	rootCmd.AddCommand(qcmd.QueueCmd)
	rootCmd.AddCommand(tccmd.TestcaseCmd)
	rootCmd.AddCommand(storecmd.StoreCmd)
	rootCmd.AddCommand(bbcmd.BlackboardCmd)
	rootCmd.AddCommand(stickcmd.StickieCmd)
	rootCmd.AddCommand(stickrelcmd.StickieRelCmd)
	rootCmd.AddCommand(scripcmd.ScriptCmd)
	rootCmd.AddCommand(message.MessageCmd)
	rootCmd.AddCommand(wscmd.WorkspaceCmd)
	rootCmd.AddCommand(role.RoleCmd)
	rootCmd.AddCommand(prjcmd.ProjectCmd)
	rootCmd.AddCommand(tagcmd.TagCmd)
	rootCmd.AddCommand(topiccmd.TopicCmd)
	rootCmd.AddCommand(pkgcmd.PackageCmd)
	rootCmd.AddCommand(workflow.WorkflowCmd)
	rootCmd.AddCommand(task.TaskCmd)
	rootCmd.AddCommand(experiment.ExperimentCmd)
	rootCmd.AddCommand(srvcmd.ServerCmd)
	rootCmd.AddCommand(snapcmd.SnapshotCmd)
	rootCmd.AddCommand(vaultcmd.VaultCmd)
	rootCmd.AddCommand(toolcmd.ToolCmd)
	rootCmd.AddCommand(promptcmd.PromptCmd)
}
