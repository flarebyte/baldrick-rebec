package cmd

import (
	"github.com/spf13/cobra"
	"github.com/your-org/your-cli/cmd/admin"
	"github.com/your-org/your-cli/cmd/test"
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
	rootCmd.AddCommand(admin.AdminCmd)
}
