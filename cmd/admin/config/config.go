package configcmd

import (
    "github.com/spf13/cobra"
)

var ConfigCmd = &cobra.Command{
    Use:   "config",
    Short: "Manage global configuration (~/.baldrick-rebec/config.yaml)",
}

func init() {
    ConfigCmd.AddCommand(initCmd)
    ConfigCmd.AddCommand(printCmd)
    ConfigCmd.AddCommand(validateCmd)
}

