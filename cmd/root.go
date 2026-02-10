package cmd

import (
	"github.com/flarebyte/baldrick-rebec/cli"
	bbcmd "github.com/flarebyte/baldrick-rebec/cmd/blackboard"
	configcmd "github.com/flarebyte/baldrick-rebec/cmd/config"
	"github.com/flarebyte/baldrick-rebec/cmd/conversation"
	dbcmd "github.com/flarebyte/baldrick-rebec/cmd/db"
	"github.com/flarebyte/baldrick-rebec/cmd/experiment"
	"github.com/flarebyte/baldrick-rebec/cmd/message"
	pkgcmd "github.com/flarebyte/baldrick-rebec/cmd/package"
	prjcmd "github.com/flarebyte/baldrick-rebec/cmd/project"
	promptcmd "github.com/flarebyte/baldrick-rebec/cmd/prompt"
	qcmd "github.com/flarebyte/baldrick-rebec/cmd/queue"
	"github.com/flarebyte/baldrick-rebec/cmd/role"
	scripcmd "github.com/flarebyte/baldrick-rebec/cmd/script"
	srvcmd "github.com/flarebyte/baldrick-rebec/cmd/server"
	snapcmd "github.com/flarebyte/baldrick-rebec/cmd/snapshot"
	stickcmd "github.com/flarebyte/baldrick-rebec/cmd/stickie"
	stickrelcmd "github.com/flarebyte/baldrick-rebec/cmd/stickie_rel"
	tagcmd "github.com/flarebyte/baldrick-rebec/cmd/tag"
	"github.com/flarebyte/baldrick-rebec/cmd/task"
	"github.com/flarebyte/baldrick-rebec/cmd/test"
	tccmd "github.com/flarebyte/baldrick-rebec/cmd/testcase"
	toolcmd "github.com/flarebyte/baldrick-rebec/cmd/tool"
	vaultcmd "github.com/flarebyte/baldrick-rebec/cmd/vault"
	vercmd "github.com/flarebyte/baldrick-rebec/cmd/version"
	"github.com/flarebyte/baldrick-rebec/cmd/workflow"
	wscmd "github.com/flarebyte/baldrick-rebec/cmd/workspace"
	"github.com/flarebyte/baldrick-rebec/internal/buildinfo"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rbc",
	Short: "Autonomous build automation and blackboard CLI",
	Long:  helpIntro,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Expose --version and the version subcommand
	// Cobra adds --version when this is non-empty
	v := buildinfo.Summary()
	if v == "dev" && cli.Version != "" {
		v = cli.Version
	}
	rootCmd.Version = v
	rootCmd.AddCommand(vercmd.VersionCmd)

	rootCmd.AddCommand(test.TestCmd)
	// Expose former `admin` subcommands at the root level
	rootCmd.AddCommand(conversation.ConversationCmd)
	rootCmd.AddCommand(configcmd.ConfigCmd)
	rootCmd.AddCommand(dbcmd.DBCmd)
	rootCmd.AddCommand(qcmd.QueueCmd)
	rootCmd.AddCommand(tccmd.TestcaseCmd)
	rootCmd.AddCommand(bbcmd.BlackboardCmd)
	rootCmd.AddCommand(stickcmd.StickieCmd)
	rootCmd.AddCommand(stickrelcmd.StickieRelCmd)
	rootCmd.AddCommand(scripcmd.ScriptCmd)
	rootCmd.AddCommand(message.MessageCmd)
	rootCmd.AddCommand(wscmd.WorkspaceCmd)
	rootCmd.AddCommand(role.RoleCmd)
	rootCmd.AddCommand(prjcmd.ProjectCmd)
	rootCmd.AddCommand(tagcmd.TagCmd)
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

const helpIntro = `rbc - autonomous build automation and blackboard CLI
Also known as baldrick-rebec, a command-line tool for task/workflow automation on Postgres; managing blackboards & stickies with folder sync, diff, and import; scripts/messages and queues; and schema-aware snapshot backups.
Copyright (c) 2026 Flarebyte.com - MIT License`
