package stickie_rel

import (
    "context"
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
    flagRelFrom   string
    flagRelTo     string
    flagRelType   string
    flagRelLabels []string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a relationship between two stickies",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagRelFrom) == "" { return errors.New("--from is required") }
        if strings.TrimSpace(flagRelTo) == "" { return errors.New("--to is required") }
        if strings.TrimSpace(flagRelType) == "" { return errors.New("--type is required") }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        // normalize labels: allow comma-separated in a single flag
        labels := splitCSV(flagRelLabels)
        // Try graph first; if it fails, fall back to SQL mirror
        // Try graph first; control fallback via config
        allowFallback := cfg.Graph.AllowFallback
        if err := pgdao.CreateStickieEdge(ctx, db, flagRelFrom, flagRelTo, flagRelType, labels); err != nil {
            if allowFallback {
                fmt.Fprintf(os.Stderr, "warn: graph edge creation failed: %v; falling back to SQL mirror\n", err)
            } else {
                return err
            }
        }
        // If fallback is disabled, require graph verification to succeed
        if !allowFallback {
            if rel, err := pgdao.GetStickieEdge(ctx, db, flagRelFrom, flagRelTo, flagRelType); err != nil {
                return fmt.Errorf("graph verify failed: %w", err)
            } else if rel == nil {
                return fmt.Errorf("graph relation not found after creation (from=%s to=%s type=%s)", flagRelFrom, flagRelTo, flagRelType)
            }
        } else {
            // Fallback allowed: if graph missing, mirror into SQL
            if rel, err := pgdao.GetStickieEdge(ctx, db, flagRelFrom, flagRelTo, flagRelType); err != nil || rel == nil {
                if err := pgdao.UpsertStickieRelation(ctx, db, pgdao.StickieRelation{FromID: flagRelFrom, ToID: flagRelTo, RelType: strings.ToUpper(flagRelType), Labels: labels}); err != nil {
                    return fmt.Errorf("sql mirror upsert failed: %w", err)
                }
                if _, gerr := pgdao.GetStickieRelation(ctx, db, flagRelFrom, flagRelTo, strings.ToUpper(flagRelType)); gerr != nil {
                    return fmt.Errorf("relation not found after creation in SQL mirror: %w", gerr)
                }
            }
        }
        fmt.Fprintf(os.Stderr, "stickie relation set from=%s to=%s type=%s\n", flagRelFrom, flagRelTo, flagRelType)
        fmt.Fprintf(os.Stdout, "{\n  \"status\": \"upserted\", \"from\": \"%s\", \"to\": \"%s\", \"type\": \"%s\"\n}\n", flagRelFrom, flagRelTo, flagRelType)
        return nil
    },
}

func init() {
    StickieRelCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagRelFrom, "from", "", "From stickie UUID (required)")
    setCmd.Flags().StringVar(&flagRelTo, "to", "", "To stickie UUID (required)")
    setCmd.Flags().StringVar(&flagRelType, "type", "", "Relation type: includes|causes|uses|represents|contrasts_with")
    setCmd.Flags().StringSliceVar(&flagRelLabels, "labels", nil, "Labels (repeat or comma-separated)")
}

func splitCSV(items []string) []string {
    if len(items) == 0 { return nil }
    out := []string{}
    for _, it := range items {
        for _, p := range strings.Split(it, ",") {
            p = strings.TrimSpace(p); if p != "" { out = append(out, p) }
        }
    }
    return out
}
