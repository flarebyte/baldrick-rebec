package oscmd

import (
    "github.com/spf13/cobra"
)

var OSCmd = &cobra.Command{
    Use:   "os",
    Short: "OpenSearch administration commands",
}

func init() {
    OSCmd.AddCommand(ilmCmd)
}

