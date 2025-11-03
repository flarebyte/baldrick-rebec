package pkg

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
    tt "text/tabwriter"
    "github.com/spf13/cobra"
)

var (
    flagPkgListRoleName string
    flagPkgListVariant  string
    flagPkgListLimit    int
    flagPkgListOffset   int
    flagPkgListOutput   string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List packages for a role (required)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagPkgListRoleName) == "" {
            return errors.New("--role is required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        items, err := pgdao.ListPackages(ctx, db, flagPkgListRoleName, flagPkgListVariant, flagPkgListLimit, flagPkgListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "packages: %d\n", len(items))
        if strings.ToLower(strings.TrimSpace(flagPkgListOutput)) == "json" {
            arr := make([]map[string]any, 0, len(items))
            for _, p := range items {
                m := map[string]any{"id": p.ID, "role_name": p.RoleName, "task_id": p.TaskID}
                if p.Created.Valid { m["created"] = p.Created.Time.Format(time.RFC3339Nano) }
                if p.Updated.Valid { m["updated"] = p.Updated.Time.Format(time.RFC3339Nano) }
                arr = append(arr, m)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        tw := tt.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(tw, "ID\tROLE\tTASK_ID")
        for _, p := range items { fmt.Fprintf(tw, "%s\t%s\t%s\n", p.ID, p.RoleName, p.TaskID) }
        tw.Flush(); return nil
    },
}

func init() {
    PackageCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagPkgListRoleName, "role", "", "Role name (required)")
    listCmd.Flags().StringVar(&flagPkgListVariant, "variant", "", "Filter by variant")
    listCmd.Flags().IntVar(&flagPkgListLimit, "limit", 100, "Max rows")
    listCmd.Flags().IntVar(&flagPkgListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().StringVar(&flagPkgListOutput, "output", "table", "Output format: table or json")
}
