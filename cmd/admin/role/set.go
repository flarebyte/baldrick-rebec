package role

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
    flagRoleName  string
    flagRoleTitle string
    flagRoleDesc  string
    flagRoleNotes string
    flagRoleTags  []string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a role (by name)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagRoleName) == "" || strings.TrimSpace(flagRoleTitle) == "" {
            return errors.New("--name and --title are required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        r := &pgdao.Role{Name: flagRoleName, Title: flagRoleTitle}
        if flagRoleDesc != "" { r.Description = sql.NullString{String: flagRoleDesc, Valid: true} }
        if flagRoleNotes != "" { r.Notes = sql.NullString{String: flagRoleNotes, Valid: true} }
        if len(flagRoleTags) > 0 { r.Tags = parseTags(flagRoleTags) }
        if err := pgdao.UpsertRole(ctx, db, r); err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "role upserted name=%q title=%q\n", r.Name, r.Title)
        // JSON
        out := map[string]any{
            "status": "upserted",
            "name":   r.Name,
            "title":  r.Title,
        }
        if r.Description.Valid { out["description"] = r.Description.String }
        if r.Notes.Valid { out["notes"] = r.Notes.String }
        if len(r.Tags) > 0 { out["tags"] = r.Tags }
        if r.Created.Valid { out["created"] = r.Created.Time.Format(time.RFC3339Nano) }
        if r.Updated.Valid { out["updated"] = r.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    RoleCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagRoleName, "name", "", "Role name (required)")
    setCmd.Flags().StringVar(&flagRoleTitle, "title", "", "Role title (required)")
    setCmd.Flags().StringVar(&flagRoleDesc, "description", "", "Role description")
    setCmd.Flags().StringVar(&flagRoleNotes, "notes", "", "Notes (markdown)")
    setCmd.Flags().StringSliceVar(&flagRoleTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
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
