package task

import (
    "context"
    "encoding/json"
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
    flagTaskListWF     string
    flagTaskListLimit  int
    flagTaskListOffset int
    flagTaskListMax    int
    flagTaskListOutput string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List tasks (optionally filter by workflow)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        effLimit := flagTaskListMax
        if effLimit <= 0 { effLimit = flagTaskListLimit }
        tasks, err := pgdao.ListTasks(ctx, db, flagTaskListWF, effLimit, flagTaskListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "tasks: %d\n", len(tasks))
        if strings.ToLower(strings.TrimSpace(flagTaskListOutput)) == "json" {
            arr := make([]map[string]any, 0, len(tasks))
            for _, t := range tasks {
                item := map[string]any{"id": t.ID, "workflow": t.WorkflowID, "command": t.Command, "variant": t.Variant, "version": t.Version}
                if t.Created.Valid { item["created"] = t.Created.Time.Format(time.RFC3339Nano) }
                if t.Title.Valid { item["title"] = t.Title.String }
                if len(t.Tags) > 0 { item["tags"] = t.Tags }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        tw := tt.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(tw, "ID\tVARIANT\tVERSION\tCOMMAND")
        for _, t := range tasks { fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.ID, t.Variant, t.Version, t.Command) }
        tw.Flush(); return nil
    },
}

func init() {
    TaskCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagTaskListWF, "workflow", "", "Filter by workflow name")
    listCmd.Flags().IntVar(&flagTaskListLimit, "limit", 100, "Max rows (deprecated; prefer --max-results)")
    listCmd.Flags().IntVar(&flagTaskListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().IntVar(&flagTaskListMax, "max-results", 20, "Max results to return (default 20)")
    listCmd.Flags().StringVar(&flagTaskListOutput, "output", "table", "Output format: table or json")
}
