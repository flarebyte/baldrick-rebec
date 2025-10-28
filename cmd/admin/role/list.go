package role

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagRoleListLimit  int
    flagRoleListOffset int
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List roles (paginated)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        roles, err := pgdao.ListRoles(ctx, db, flagRoleListLimit, flagRoleListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "roles: %d\n", len(roles))
        arr := make([]map[string]any, 0, len(roles))
        for _, r := range roles {
            item := map[string]any{
                "name":  r.Name,
                "title": r.Title,
            }
            if r.Description.Valid { item["description"] = r.Description.String }
            if r.Notes.Valid { item["notes"] = r.Notes.String }
            if len(r.Tags) > 0 { item["tags"] = r.Tags }
            if r.Created.Valid { item["created"] = r.Created.Time.Format(time.RFC3339Nano) }
            if r.Updated.Valid { item["updated"] = r.Updated.Time.Format(time.RFC3339Nano) }
            arr = append(arr, item)
        }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(arr)
    },
}

func init() {
    RoleCmd.AddCommand(listCmd)
    listCmd.Flags().IntVar(&flagRoleListLimit, "limit", 100, "Max rows")
    listCmd.Flags().IntVar(&flagRoleListOffset, "offset", 0, "Offset for pagination")
}

