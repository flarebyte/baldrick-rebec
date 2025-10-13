package conversation

import (
	"fmt"
	"github.com/spf13/cobra"
)

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "TODO: Describe the 'admin conversation join' command",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement join logic
		fmt.Println("Joining conversation...")
	},
}
