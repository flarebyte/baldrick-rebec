package cmd

import (
	"github.com/flarebyte/baldrick-rebec/cmd/admin"
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
	rootCmd.AddCommand(admin.AdminCmd)
}
