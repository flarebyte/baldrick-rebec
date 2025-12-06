package vaultcmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	vpkg "github.com/flarebyte/baldrick-rebec/internal/vault"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List secrets stored in the vault (no values)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		dao, err := vpkg.NewVaultDAO(cfg.Vault.Backend)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		items, err := dao.ListSecrets(ctx)
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		// Output: name [set|unset] backend
		for _, it := range items {
			status := "unset"
			if it.IsSet {
				status = "set"
			}
			backend := it.Backend
			if backend == "" {
				backend = cfg.Vault.Backend
			}
			fmt.Fprintf(os.Stdout, "%s\t[%s]\t%s\n", strings.TrimSpace(it.Name), status, backend)
		}
		return nil
	},
}

func init() {
	VaultCmd.AddCommand(listCmd)
}
