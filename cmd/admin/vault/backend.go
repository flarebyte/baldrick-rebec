package vaultcmd

import (
	"errors"
	"fmt"
	"os"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	"github.com/flarebyte/baldrick-rebec/internal/paths"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Manage the secret storage backend",
}

var backendCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the current backend",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		if cfg.Vault.Backend == "" {
			fmt.Fprintln(os.Stdout, "keychain")
			return nil
		}
		fmt.Fprintln(os.Stdout, cfg.Vault.Backend)
		return nil
	},
}

var backendListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backends",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := cfgpkg.Load()
		cur := cfg.Vault.Backend
		if cur == "" {
			cur = "keychain"
		}
		fmt.Fprintf(os.Stdout, "keychain (default, enabled)%s\n", markCurrent(cur == "keychain"))
		fmt.Fprintf(os.Stdout, "yubikey (not implemented)%s\n", markCurrent(cur == "yubikey"))
		return nil
	},
}

func markCurrent(is bool) string {
	if is {
		return "  [current]"
	}
	return ""
}

var backendSetCmd = &cobra.Command{
	Use:   "set <backend>",
	Short: "Set the active backend (only 'keychain' supported)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		be := args[0]
		switch be {
		case "keychain":
			// supported, proceed
		default:
			return errors.New("backend not implemented")
		}
		if _, err := paths.EnsureHome(); err != nil {
			return err
		}
		path := cfgpkg.Path()
		cfg, _ := cfgpkg.Load()
		cfg.Vault.Backend = be
		b, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "backend set to %q in %s\n", be, path)
		return nil
	},
}

func init() {
	VaultCmd.AddCommand(backendCmd)
	backendCmd.AddCommand(backendCurrentCmd)
	backendCmd.AddCommand(backendListCmd)
	backendCmd.AddCommand(backendSetCmd)
}
