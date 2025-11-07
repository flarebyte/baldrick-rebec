package message

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
    flagMsgListExperiment string
    flagMsgListTask       string
    flagMsgListStatus     string
    flagMsgListLimit      int
    flagMsgListOffset     int
    flagMsgListMax        int
    flagMsgListOutput     string
    flagMsgListRole       string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List messages (filter by experiment, task, or status)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        effLimit := flagMsgListMax
        if effLimit <= 0 { effLimit = flagMsgListLimit }
        if strings.TrimSpace(flagMsgListRole) == "" { return errors.New("--role is required") }
        ms, err := pgdao.ListMessages(ctx, db, flagMsgListRole, flagMsgListExperiment, flagMsgListTask, flagMsgListStatus, effLimit, flagMsgListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "messages: %d\n", len(ms))
        if strings.ToLower(strings.TrimSpace(flagMsgListOutput)) == "json" {
            arr := make([]map[string]any, 0, len(ms))
            for _, m := range ms {
                item := map[string]any{"id": m.ID, "content_id": m.ContentID, "status": m.Status, "created": m.Created.Format(time.RFC3339Nano)}
                if m.FromTaskID.Valid { item["from_task_id"] = m.FromTaskID.String }
                if m.ExperimentID.Valid { item["experiment_id"] = m.ExperimentID.String }
                if len(m.Tags) > 0 { item["tags"] = m.Tags }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        table := tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"ID", "STATUS", "CREATED"})
        for _, m := range ms { table.Append([]string{m.ID, m.Status, m.Created.Format(time.RFC3339)}) }
        table.Render(); return nil
    },
}

func init() {
    MessageCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagMsgListExperiment, "experiment", "", "Filter by experiment UUID")
    listCmd.Flags().StringVar(&flagMsgListTask, "task", "", "Filter by task UUID")
    listCmd.Flags().StringVar(&flagMsgListStatus, "status", "", "Filter by status")
    listCmd.Flags().IntVar(&flagMsgListLimit, "limit", 100, "Max rows (deprecated; prefer --max-results)")
    listCmd.Flags().IntVar(&flagMsgListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().IntVar(&flagMsgListMax, "max-results", 20, "Max results to return (default 20)")
    listCmd.Flags().StringVar(&flagMsgListOutput, "output", "table", "Output format: table or json")
    listCmd.Flags().StringVar(&flagMsgListRole, "role", "", "Role name (required)")
}
