package task

import "github.com/spf13/cobra"

var TaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks (create/update/list/get/delete)",
}
