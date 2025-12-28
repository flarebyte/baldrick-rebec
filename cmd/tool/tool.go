package tool

import "github.com/spf13/cobra"

var ToolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Manage tools (role-scoped catalog)",
}
