package store

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
    flagStoreGetName string
    flagStoreGetRole string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a store by name and role",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagStoreGetName) == "" { return errors.New("--name is required") }
        if strings.TrimSpace(flagStoreGetRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        s, err := pgdao.GetStoreByKey(ctx, db, flagStoreGetName, flagStoreGetRole)
        if err != nil { return err }
        // stderr line
        fmt.Fprintf(os.Stderr, "store name=%q role=%q id=%s\n", s.Name, s.RoleName, s.ID)
        // stdout JSON
        out := map[string]any{
            "id":     s.ID,
            "name":   s.Name,
            "role":   s.RoleName,
            "title":  s.Title,
        }
        if s.Description.Valid && s.Description.String != "" { out["description"] = s.Description.String }
        if s.Motivation.Valid && s.Motivation.String != "" { out["motivation"] = s.Motivation.String }
        if s.Security.Valid && s.Security.String != "" { out["security"] = s.Security.String }
        if s.Privacy.Valid && s.Privacy.String != "" { out["privacy"] = s.Privacy.String }
        if s.Notes.Valid && s.Notes.String != "" { out["notes"] = s.Notes.String }
        if s.StoreType.Valid && s.StoreType.String != "" { out["type"] = s.StoreType.String }
        if s.Scope.Valid && s.Scope.String != "" { out["scope"] = s.Scope.String }
        if s.Lifecycle.Valid && s.Lifecycle.String != "" { out["lifecycle"] = s.Lifecycle.String }
        if s.Created.Valid { out["created"] = s.Created.Time.Format(time.RFC3339Nano) }
        if s.Updated.Valid { out["updated"] = s.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    StoreCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagStoreGetName, "name", "", "Store unique name within role (required)")
    getCmd.Flags().StringVar(&flagStoreGetRole, "role", "", "Role name (required)")
}

