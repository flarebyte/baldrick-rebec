package script

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
    "github.com/olekukonko/tablewriter"
    "github.com/spf13/cobra"
)

var (
    flagScrListLimit  int
    flagScrListOffset int
    flagScrListOutput string
    flagScrListRole   string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List scripts for a role (paginated)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagScrListRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        ss, err := pgdao.ListScripts(ctx, db, flagScrListRole, flagScrListLimit, flagScrListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "scripts: %d\n", len(ss))
        out := strings.ToLower(strings.TrimSpace(flagScrListOutput))
        if out == "json" {
            arr := make([]map[string]any, 0, len(ss))
            for _, s := range ss {
                item := map[string]any{"id": s.ID, "title": s.Title, "role": s.RoleName}
                if s.Created.Valid { item["created"] = s.Created.Time.Format(time.RFC3339Nano) }
                if s.Updated.Valid { item["updated"] = s.Updated.Time.Format(time.RFC3339Nano) }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        // table default
        table := tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"ID", "TITLE", "UPDATED"})
        for _, s := range ss {
            updated := ""; if s.Updated.Valid { updated = s.Updated.Time.Format(time.RFC3339) }
            table.Append([]string{s.ID, s.Title, updated})
        }
        table.Render()
        return nil
    },
}

func init() {
    ScriptCmd.AddCommand(listCmd)
    listCmd.Flags().IntVar(&flagScrListLimit, "limit", 100, "Max number of rows")
    listCmd.Flags().IntVar(&flagScrListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().StringVar(&flagScrListOutput, "output", "table", "Output format: table or json")
    listCmd.Flags().StringVar(&flagScrListRole, "role", "", "Role name (required)")
}
