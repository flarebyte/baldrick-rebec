package oscmd

import (
    "fmt"
    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/spf13/cobra"
)

var OSCmd = &cobra.Command{
    Use:   "os",
    Short: "OpenSearch administration commands",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        cfg, _ := cfgpkg.Load()
        if cfg.Features.PGOnly {
            return fmt.Errorf("pg_only=true: OpenSearch commands are disabled")
        }
        return nil
    },
}

func init() {
    OSCmd.AddCommand(ilmCmd)
}
