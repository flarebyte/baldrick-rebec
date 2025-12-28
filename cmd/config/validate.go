package configcmd

import (
	"github.com/spf13/cobra"
)

// validateCmd is a deprecated alias for `check` to preserve backwards compatibility.
var validateCmd = &cobra.Command{
	Use:    "validate",
	Short:  "Deprecated: use 'check'",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return checkCmd.RunE(cmd, args)
	},
}
