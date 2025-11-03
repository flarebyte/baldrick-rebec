package workspace

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
    flagWSGetID string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a workspace by id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagWSGetID) == "" { return errors.New("--id is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        w, err := pgdao.GetWorkspaceByID(ctx, db, flagWSGetID)
        if err != nil { return err }
        // stderr line
        fmt.Fprintf(os.Stderr, "workspace id=%s role=%q dir=%q\n", w.ID, w.RoleName, w.Directory)
        // stdout JSON
        out := map[string]any{
            "id":        w.ID,
            "role":      w.RoleName,
            "directory": w.Directory,
        }
        if w.Created.Valid { out["created"] = w.Created.Time.Format(time.RFC3339Nano) }
        if w.Updated.Valid { out["updated"] = w.Updated.Time.Format(time.RFC3339Nano) }
        if w.Description.Valid && w.Description.String != "" { out["description"] = w.Description.String }
        if w.ProjectName.Valid && w.ProjectName.String != "" { out["project"] = w.ProjectName.String }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    WorkspaceCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagWSGetID, "id", "", "Workspace UUID (required)")
}

