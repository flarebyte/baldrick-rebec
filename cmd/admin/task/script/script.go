package task

import "github.com/spf13/cobra"

// ScriptCmd is the parent for task script subcommands: add, remove, list, get
var ScriptCmd = &cobra.Command{
    Use:   "script",
    Short: "Manage scripts attached to a task",
}

func init() {
    // Attach under 'task'
    TaskCmd.AddCommand(ScriptCmd)
}

