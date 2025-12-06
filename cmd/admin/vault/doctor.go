package vaultcmd

import (
	"context"
	"fmt"
	"os"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	vpkg "github.com/flarebyte/baldrick-rebec/internal/vault"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics for the vault backend",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "backend: %s\n", cfg.Vault.Backend)
		dao, err := vpkg.NewVaultDAO(cfg.Vault.Backend)
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: ERROR (%v)\n", err)
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		items, err := dao.ListSecrets(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: ERROR (%v)\n", err)
			return err
		}
		fmt.Fprintf(os.Stderr, "keychain access: ok\n")
		fmt.Fprintf(os.Stderr, "secrets present: %d\n", len(items))
		fmt.Fprintf(os.Stderr, "status: OK\n")
		return nil
	},
}

func init() {
	VaultCmd.AddCommand(doctorCmd)
}
