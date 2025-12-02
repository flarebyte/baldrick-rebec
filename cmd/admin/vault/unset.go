package vaultcmd

import (
    "context"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    vpkg "github.com/flarebyte/baldrick-rebec/internal/vault"
    "github.com/spf13/cobra"
)

var unsetCmd = &cobra.Command{
    Use:   "unset <name>",
    Short: "Delete a secret from the vault",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := strings.TrimSpace(args[0])
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        dao, err := vpkg.NewVaultDAO(cfg.Vault.Backend)
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        if err := dao.UnsetSecret(ctx, name); err != nil { return err }
        fmt.Fprintf(os.Stderr, "secret %q deleted from backend %q\n", name, cfg.Vault.Backend)
        return nil
    },
}

func init() {
    VaultCmd.AddCommand(unsetCmd)
}

