package workspace

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagWSID     string
    flagWSRole   string
    flagWSDesc   string
    flagWSProj   string
    flagWSTags   []string
    flagWSDir    string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a workspace (by id)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagWSRole) == "" { return errors.New("--role is required") }
        if strings.TrimSpace(flagWSDir) == "" { return errors.New("--directory is required") }
        if !filepath.IsAbs(flagWSDir) {
            return errors.New("--directory must be an absolute path")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        w := &pgdao.Workspace{ ID: flagWSID, RoleName: flagWSRole, Directory: flagWSDir }
        if flagWSDesc != "" { w.Description = sql.NullString{String: flagWSDesc, Valid: true} }
        if flagWSProj != "" { w.ProjectName = sql.NullString{String: flagWSProj, Valid: true} }
        if len(flagWSTags) > 0 { w.Tags = parseTags(flagWSTags) }
        if err := pgdao.UpsertWorkspace(ctx, db, w); err != nil { return err }

        // stderr summary
        fmt.Fprintf(os.Stderr, "workspace upserted id=%s role=%q dir=%q\n", w.ID, w.RoleName, w.Directory)
        // stdout JSON
        out := map[string]any{
            "status":    "upserted",
            "id":        w.ID,
            "role":      w.RoleName,
            "directory": w.Directory,
        }
        if w.Created.Valid { out["created"] = w.Created.Time.Format(time.RFC3339Nano) }
        if w.Updated.Valid { out["updated"] = w.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    WorkspaceCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagWSID, "id", "", "Workspace UUID (optional; when omitted, a new id is generated)")
    setCmd.Flags().StringVar(&flagWSRole, "role", "", "Role name (required)")
    setCmd.Flags().StringVar(&flagWSDesc, "description", "", "Plain text description")
    setCmd.Flags().StringVar(&flagWSProj, "project", "", "Project name (must exist for role if provided)")
    setCmd.Flags().StringVar(&flagWSDir, "directory", "", "Absolute path to workspace directory (required)")
    setCmd.Flags().StringSliceVar(&flagWSTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
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

