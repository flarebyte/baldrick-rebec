package testcase

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
    flagTCListRole string
    flagTCListExperiment string
    flagTCListStatus string
    flagTCListLimit int
    flagTCListOffset int
    flagTCListOutput string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List testcases",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagTCListRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        items, err := pgdao.ListTestcases(ctx, db, flagTCListRole, flagTCListExperiment, flagTCListStatus, flagTCListLimit, flagTCListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "testcases: %d\n", len(items))
        if strings.ToLower(strings.TrimSpace(flagTCListOutput)) == "json" {
            arr := make([]map[string]any, 0, len(items))
            for _, t := range items {
                m := map[string]any{"id": t.ID, "title": t.Title, "status": t.Status}
                if t.Created.Valid { m["created"] = t.Created.Time.Format(time.RFC3339Nano) }
                if t.Name.Valid { m["name"] = t.Name.String }
                if t.Package.Valid { m["package"] = t.Package.String }
                if t.Classname.Valid { m["classname"] = t.Classname.String }
                if t.ExperimentID.Valid { m["experiment_id"] = t.ExperimentID.String }
                if len(t.Tags) > 0 { m["tags"] = t.Tags }
                arr = append(arr, m)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        table := tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"ID", "TITLE", "STATUS", "CREATED"})
        for _, t := range items {
            created := ""; if t.Created.Valid { created = t.Created.Time.Format(time.RFC3339) }
            table.Append([]string{t.ID, t.Title, t.Status, created})
        }
        table.Render(); return nil
    },
}

func init() {
    TestcaseCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagTCListRole, "role", "", "Role name (required)")
    listCmd.Flags().StringVar(&flagTCListExperiment, "experiment", "", "Filter by experiment UUID")
    listCmd.Flags().StringVar(&flagTCListStatus, "status", "", "Filter by status")
    listCmd.Flags().IntVar(&flagTCListLimit, "limit", 100, "Max rows")
    listCmd.Flags().IntVar(&flagTCListOffset, "offset", 0, "Offset")
    listCmd.Flags().StringVar(&flagTCListOutput, "output", "table", "Output format: table or json")
}
