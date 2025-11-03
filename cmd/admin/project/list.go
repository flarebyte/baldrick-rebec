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
    tt "text/tabwriter"
    "github.com/spf13/cobra"
)

var (
    flagPrjListLimit  int
    flagPrjListOffset int
    flagPrjListOutput string
    flagPrjListRole   string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List projects for a role (paginated)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagPrjListRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        ps, err := pgdao.ListProjects(ctx, db, flagPrjListRole, flagPrjListLimit, flagPrjListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "projects: %d\n", len(ps))
        out := strings.ToLower(strings.TrimSpace(flagPrjListOutput))
        if out == "json" {
            arr := make([]map[string]any, 0, len(ps))
            for _, p := range ps {
                item := map[string]any{"name": p.Name, "role": p.RoleName}
                if p.Created.Valid { item["created"] = p.Created.Time.Format(time.RFC3339Nano) }
                if p.Updated.Valid { item["updated"] = p.Updated.Time.Format(time.RFC3339Nano) }
                if p.Description.Valid && p.Description.String != "" { item["description"] = p.Description.String }
                if p.Notes.Valid && p.Notes.String != "" { item["notes"] = p.Notes.String }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        // table default
        tw := tt.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(tw, "NAME\tUPDATED")
        for _, p := range ps {
            updated := ""; if p.Updated.Valid { updated = p.Updated.Time.Format(time.RFC3339) }
            fmt.Fprintf(tw, "%s\t%s\n", p.Name, updated)
        }
        tw.Flush()
        return nil
    },
}

func init() {
    ProjectCmd.AddCommand(listCmd)
    listCmd.Flags().IntVar(&flagPrjListLimit, "limit", 100, "Max number of rows")
    listCmd.Flags().IntVar(&flagPrjListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().StringVar(&flagPrjListOutput, "output", "table", "Output format: table or json")
    listCmd.Flags().StringVar(&flagPrjListRole, "role", "", "Role name (required)")
}

