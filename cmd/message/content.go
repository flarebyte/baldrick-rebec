package message

import "github.com/spf13/cobra"

var ContentCmd = &cobra.Command{
	Use:   "content",
	Short: "Work with message content blobs",
}

func init() {
	MessageCmd.AddCommand(ContentCmd)
}
