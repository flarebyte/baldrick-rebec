package message

import (
    "github.com/spf13/cobra"
)

var MessageCmd = &cobra.Command{
    Use:   "message",
    Short: "TODO: Describe the 'admin message' command",
}

func init() {
    MessageCmd.AddCommand(setCmd)
}
