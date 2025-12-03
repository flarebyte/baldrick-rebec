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

var showCmd = &cobra.Command{
    Use:   "show <name>",
    Short: "Show metadata about a secret (never prints the value)",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        name := args[0]
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        dao, err := vpkg.NewVaultDAO(cfg.Vault.Backend)
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        md, err := dao.GetSecretMetadata(ctx, name)
        if err != nil { return err }
        status := "unset"; if md.IsSet { status = "set" }
        backend := md.Backend; if backend == "" { backend = cfg.Vault.Backend }
        fmt.Fprintf(os.Stdout, "Name: %s\n", md.Name)
        fmt.Fprintf(os.Stdout, "Status: %s\n", status)
        fmt.Fprintf(os.Stdout, "Backend: %s\n", backend)
        if md.UpdatedAt != nil { fmt.Fprintf(os.Stdout, "Last Updated: %s\n", md.UpdatedAt.Format(time.RFC3339)) }
        return nil
    },
}

func init() {
    VaultCmd.AddCommand(showCmd)
}

