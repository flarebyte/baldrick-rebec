package queue

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagQDesc string
    flagQStatus string
    flagQWhy string
    flagQTags []string
    flagQTask string
    flagQInbound string
    flagQWorkspace string
)

var addCmd = &cobra.Command{
    Use:   "add",
    Short: "Add a new queue record",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        q := &pgdao.Queue{ Status: strings.TrimSpace(flagQStatus) }
        if flagQDesc != "" { q.Description = sql.NullString{String: flagQDesc, Valid: true} }
        if flagQWhy != "" { q.Why = sql.NullString{String: flagQWhy, Valid: true} }
        if len(flagQTags) > 0 { q.Tags = parseTags(flagQTags) }
        if strings.TrimSpace(flagQTask) != "" { q.TaskID = sql.NullString{String: flagQTask, Valid: true} }
        if strings.TrimSpace(flagQInbound) != "" { q.InboundMessageID = sql.NullString{String: flagQInbound, Valid: true} }
        if strings.TrimSpace(flagQWorkspace) != "" { q.TargetWorkspaceID = sql.NullString{String: flagQWorkspace, Valid: true} }
        if err := pgdao.AddQueue(ctx, db, q); err != nil { return err }
        fmt.Fprintf(os.Stderr, "queue added id=%s status=%s\n", q.ID, q.Status)
        out := map[string]any{"id": q.ID, "status": q.Status}
        if q.InQueueSince.Valid { out["inQueueSince"] = q.InQueueSince.Time.Format(time.RFC3339Nano) }
        if q.Description.Valid { out["description"] = q.Description.String }
        if q.Why.Valid { out["why"] = q.Why.String }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    QueueCmd.AddCommand(addCmd)
    addCmd.Flags().StringVar(&flagQDesc, "description", "", "Description")
    addCmd.Flags().StringVar(&flagQStatus, "status", "Waiting", "Status: Waiting|Blocked|Buildable|Running|Completed")
    addCmd.Flags().StringVar(&flagQWhy, "why", "", "Why still queued")
    addCmd.Flags().StringSliceVar(&flagQTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
    addCmd.Flags().StringVar(&flagQTask, "task-id", "", "Related task UUID")
    addCmd.Flags().StringVar(&flagQInbound, "inbound-message", "", "Inbound message UUID")
    addCmd.Flags().StringVar(&flagQWorkspace, "target-workspace", "", "Target workspace UUID")
}

// parseTags converts k=v pairs (or bare keys) into a map.
func parseTags(items []string) map[string]any {
    if len(items) == 0 { return nil }
    out := map[string]any{}
    for _, raw := range items {
        if raw == "" { continue }
        parts := strings.Split(raw, ",")
        for _, p := range parts {
            p = strings.TrimSpace(p)
            if p == "" { continue }
            if eq := strings.IndexByte(p, '='); eq > 0 {
                k := strings.TrimSpace(p[:eq])
                v := strings.TrimSpace(p[eq+1:])
                if k != "" { out[k] = v }
            } else {
                out[p] = true
            }
        }
    }
    return out
}
