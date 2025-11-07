package stickie_rel

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagRelGetFrom string
    flagRelGetTo   string
    flagRelGetType string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a specific stickie relationship (from,to,type)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagRelGetFrom) == "" || strings.TrimSpace(flagRelGetTo) == "" || strings.TrimSpace(flagRelGetType) == "" {
            return errors.New("--from, --to and --type are required")
        }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        rel, err := pgdao.GetStickieEdge(ctx, db, flagRelGetFrom, flagRelGetTo, flagRelGetType)
        if err != nil { return err }
        if rel == nil { return fmt.Errorf("relationship not found") }
        out := map[string]any{"from": rel.FromID, "to": rel.ToID, "type": rel.Type, "labels": rel.Labels}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    StickieRelCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagRelGetFrom, "from", "", "From stickie UUID (required)")
    getCmd.Flags().StringVar(&flagRelGetTo, "to", "", "To stickie UUID (required)")
    getCmd.Flags().StringVar(&flagRelGetType, "type", "", "Relation type: includes|causes|uses|represents|contrasts_with")
}

