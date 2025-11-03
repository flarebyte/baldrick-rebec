package project

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
    flagPrjGetName string
    flagPrjGetRole string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a project by name and role",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagPrjGetName) == "" { return errors.New("--name is required") }
        if strings.TrimSpace(flagPrjGetRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        p, err := pgdao.GetProjectByKey(ctx, db, flagPrjGetName, flagPrjGetRole)
        if err != nil { return err }
        // stderr line
        fmt.Fprintf(os.Stderr, "project name=%q role=%q\n", p.Name, p.RoleName)
        // stdout JSON
        out := map[string]any{
            "name":  p.Name,
            "role":  p.RoleName,
        }
        if p.Created.Valid { out["created"] = p.Created.Time.Format(time.RFC3339Nano) }
        if p.Updated.Valid { out["updated"] = p.Updated.Time.Format(time.RFC3339Nano) }
        if p.Description.Valid && p.Description.String != "" { out["description"] = p.Description.String }
        if p.Notes.Valid && p.Notes.String != "" { out["notes"] = p.Notes.String }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ProjectCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagPrjGetName, "name", "", "Project unique name within role (required)")
    getCmd.Flags().StringVar(&flagPrjGetRole, "role", "", "Role name (required)")
}

