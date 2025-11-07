package stickie_rel

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
    flagRelListID    string
    flagRelListDir   string
    flagRelListTypes []string
    flagRelListOutput string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List stickie relationships for a node (out|in|both)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagRelListID) == "" { return errors.New("--id is required") }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        types := splitCSV(flagRelListTypes)
        rows, err := pgdao.ListStickieEdges(ctx, db, flagRelListID, flagRelListDir, types)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "relations: %d\n", len(rows))
        if strings.ToLower(strings.TrimSpace(flagRelListOutput)) == "json" {
            arr := make([]map[string]any, 0, len(rows))
            for _, r := range rows {
                arr = append(arr, map[string]any{"from": r.FromID, "to": r.ToID, "type": r.Type, "labels": r.Labels})
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        table := tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"FROM", "TYPE", "TO", "LABELS"})
        for _, r := range rows { table.Append([]string{r.FromID, r.Type, r.ToID, strings.Join(r.Labels, ",")}) }
        table.Render(); return nil
    },
}

func init() {
    StickieRelCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagRelListID, "id", "", "Stickie UUID to list relations for (required)")
    listCmd.Flags().StringVar(&flagRelListDir, "direction", "out", "Direction: out|in|both")
    listCmd.Flags().StringSliceVar(&flagRelListTypes, "types", nil, "Filter by relation types (comma or repeat)")
    listCmd.Flags().StringVar(&flagRelListOutput, "output", "table", "Output: table|json")
}

