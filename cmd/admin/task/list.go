package task

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
    flagTaskListWF     string
    flagTaskListLimit  int
    flagTaskListOffset int
    flagTaskListMax    int
    flagTaskListOutput string
    flagTaskListRole   string
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
        if strings.TrimSpace(flagTaskListRole) == "" { return errors.New("--role is required") }
        tasks, err := pgdao.ListTasks(ctx, db, flagTaskListWF, flagTaskListRole, effLimit, flagTaskListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "tasks: %d\n", len(tasks))
        if strings.ToLower(strings.TrimSpace(flagTaskListOutput)) == "json" {
            arr := make([]map[string]any, 0, len(tasks))
            for _, t := range tasks {
                item := map[string]any{"id": t.ID, "workflow": t.WorkflowID, "command": t.Command, "variant": t.Variant}
                if t.Created.Valid { item["created"] = t.Created.Time.Format(time.RFC3339Nano) }
                if t.Title.Valid { item["title"] = t.Title.String }
                if len(t.Tags) > 0 { item["tags"] = t.Tags }
                if t.Archived { item["archived"] = true }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        table := tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"ID", "VARIANT", "COMMAND", "ARCH"})
        for _, t := range tasks { table.Append([]string{t.ID, t.Variant, t.Command}) }
        // rewrite rows with ARCH column
        table = tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"ID", "VARIANT", "COMMAND", "ARCH"})
        for _, t := range tasks {
            arch := ""; if t.Archived { arch = "yes" }
            table.Append([]string{t.ID, t.Variant, t.Command, arch})
        }
        table.Render(); return nil
    },
}

func init() {
    TaskCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagTaskListWF, "workflow", "", "Filter by workflow name")
    listCmd.Flags().IntVar(&flagTaskListLimit, "limit", 100, "Max rows (deprecated; prefer --max-results)")
    listCmd.Flags().IntVar(&flagTaskListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().IntVar(&flagTaskListMax, "max-results", 20, "Max results to return (default 20)")
    listCmd.Flags().StringVar(&flagTaskListOutput, "output", "table", "Output format: table or json")
    listCmd.Flags().StringVar(&flagTaskListRole, "role", "", "Role name (required)")
}
