package test

import (
	"fmt"
	"github.com/spf13/cobra"
)

var unitCmd = &cobra.Command{
	Use:   "unit",
	Short: "TODO: Describe the 'test unit' command",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement unit test runner
		fmt.Println("Running unit tests...")
	},
}
