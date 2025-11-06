package blackboard

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
    flagBBListLimit  int
    flagBBListOffset int
    flagBBListOutput string
    flagBBListRole   string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List blackboards for a role (paginated)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagBBListRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        bb, err := pgdao.ListBlackboards(ctx, db, flagBBListRole, flagBBListLimit, flagBBListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "blackboards: %d\n", len(bb))
        out := strings.ToLower(strings.TrimSpace(flagBBListOutput))
        if out == "json" {
            arr := make([]map[string]any, 0, len(bb))
            for _, b := range bb {
                item := map[string]any{"id": b.ID, "role": b.RoleName, "store_id": b.StoreID}
                if b.ProjectName.Valid && b.ProjectName.String != "" { item["project"] = b.ProjectName.String }
                if b.Updated.Valid { item["updated"] = b.Updated.Time.Format(time.RFC3339Nano) }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        // table default
        tw := tt.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(tw, "ID\tSTORE\tPROJECT\tUPDATED")
        for _, b := range bb {
            updated := ""; if b.Updated.Valid { updated = b.Updated.Time.Format(time.RFC3339) }
            proj := ""; if b.ProjectName.Valid { proj = b.ProjectName.String }
            fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", b.ID, b.StoreID, proj, updated)
        }
        tw.Flush(); return nil
    },
}

func init() {
    BlackboardCmd.AddCommand(listCmd)
    listCmd.Flags().IntVar(&flagBBListLimit, "limit", 100, "Max number of rows")
    listCmd.Flags().IntVar(&flagBBListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().StringVar(&flagBBListOutput, "output", "table", "Output format: table or json")
    listCmd.Flags().StringVar(&flagBBListRole, "role", "", "Role name (required)")
}

