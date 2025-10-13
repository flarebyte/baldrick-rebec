package test

import (
	"github.com/spf13/cobra"
)

var TestCmd = &cobra.Command{
	Use:   "test",
	Short: "TODO: Describe the 'test' command",
}

func init() {
	TestCmd.AddCommand(unitCmd)
}
