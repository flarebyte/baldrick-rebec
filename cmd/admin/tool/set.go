package tool

import (
    "context"
    "database/sql"
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
    flagToolName      string
    flagToolTitle     string
    flagToolRole      string
    flagToolDesc      string
    flagToolNotes     string
    flagToolTags      []string
    flagToolSettings  string
    flagToolType      string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a tool (by name)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagToolName) == "" { return errors.New("--name is required") }
        if strings.TrimSpace(flagToolTitle) == "" { return errors.New("--title is required") }
        if strings.TrimSpace(flagToolRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()

        t := &pgdao.Tool{ Name: flagToolName, Title: flagToolTitle, RoleName: flagToolRole }
        if flagToolDesc != "" { t.Description = sql.NullString{String: flagToolDesc, Valid:true} }
        if flagToolNotes != "" { t.Notes = sql.NullString{String: flagToolNotes, Valid:true} }
        if len(flagToolTags) > 0 { t.Tags = parseTags(flagToolTags) }
        if strings.TrimSpace(flagToolSettings) != "" {
            var m map[string]any
            if err := json.Unmarshal([]byte(flagToolSettings), &m); err != nil { return fmt.Errorf("invalid --settings JSON: %w", err) }
            t.Settings = m
        }
        if strings.TrimSpace(flagToolType) != "" { t.ToolType = sql.NullString{String: flagToolType, Valid:true} }

        if err := pgdao.UpsertTool(ctx, db, t); err != nil { return err }
        fmt.Fprintf(os.Stderr, "tool upserted name=%q role=%q\n", t.Name, t.RoleName)
        out := map[string]any{"status":"upserted","name":t.Name,"role":t.RoleName}
        if t.Created.Valid { out["created"] = t.Created.Time.Format(time.RFC3339Nano) }
        if t.Updated.Valid { out["updated"] = t.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    ToolCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagToolName, "name", "", "Tool unique name (required)")
    setCmd.Flags().StringVar(&flagToolTitle, "title", "", "Tool title (required)")
    setCmd.Flags().StringVar(&flagToolRole, "role", "", "Role name (required)")
    setCmd.Flags().StringVar(&flagToolDesc, "description", "", "Tool description (optional)")
    setCmd.Flags().StringVar(&flagToolNotes, "notes", "", "Markdown notes (optional)")
    setCmd.Flags().StringSliceVar(&flagToolTags, "tags", nil, "Tags as key=value (repeat or comma-separated)")
    setCmd.Flags().StringVar(&flagToolSettings, "settings", "", "Settings as JSON object (optional)")
    setCmd.Flags().StringVar(&flagToolType, "type", "", "Tool type (optional)")
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

