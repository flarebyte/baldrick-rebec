package vaultcmd

import "github.com/spf13/cobra"

// VaultCmd is the root for `rbc admin vault` commands.
var VaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage secrets in the local vault (macOS Keychain)",
}
