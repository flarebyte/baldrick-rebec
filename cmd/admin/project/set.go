package project

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
    flagPrjName  string
    flagPrjRole  string
    flagPrjDesc  string
    flagPrjNotes string
    flagPrjTags  []string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a project (by name + role)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagPrjName) == "" { return errors.New("--name is required") }
        if strings.TrimSpace(flagPrjRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        p := &pgdao.Project{ Name: flagPrjName, RoleName: flagPrjRole }
        if flagPrjDesc != "" { p.Description = sql.NullString{String: flagPrjDesc, Valid: true} }
        if flagPrjNotes != "" { p.Notes = sql.NullString{String: flagPrjNotes, Valid: true} }
        if len(flagPrjTags) > 0 { p.Tags = parseTags(flagPrjTags) }
        if err := pgdao.UpsertProject(ctx, db, p); err != nil { return err }

        // stderr summary
        fmt.Fprintf(os.Stderr, "project upserted name=%q role=%q\n", p.Name, p.RoleName)
        // stdout JSON
        out := map[string]any{
            "status": "upserted",
            "name":   p.Name,
            "role":   p.RoleName,
        }
        if p.Created.Valid { out["created"] = p.Created.Time.Format(time.RFC3339Nano) }
        if p.Updated.Valid { out["updated"] = p.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ProjectCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagPrjName, "name", "", "Project unique name within role (required)")
    setCmd.Flags().StringVar(&flagPrjRole, "role", "", "Role name (required)")
    setCmd.Flags().StringVar(&flagPrjDesc, "description", "", "Plain text description")
    setCmd.Flags().StringVar(&flagPrjNotes, "notes", "", "Markdown-formatted notes")
    setCmd.Flags().StringSliceVar(&flagPrjTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
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

