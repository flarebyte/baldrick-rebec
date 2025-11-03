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
    tt "text/tabwriter"
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
                item := map[string]any{
                    "id": m.ID,
                    "content_id": m.ContentID,
                    "status": m.Status,
                    "received_at": m.ReceivedAt.Format(time.RFC3339Nano),
                }
                if m.TaskID.Valid { item["task_id"] = m.TaskID.String }
                if m.ExperimentID.Valid { item["experiment_id"] = m.ExperimentID.String }
                if len(m.Tags) > 0 { item["tags"] = m.Tags }
                if m.Executor.Valid { item["executor"] = m.Executor.String }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        tw := tt.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(tw, "ID\tSTATUS\tRECEIVED\tEXECUTOR")
        for _, m := range ms {
            ex := ""; if m.Executor.Valid { ex = m.Executor.String }
            fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", m.ID, m.Status, m.ReceivedAt.Format(time.RFC3339), ex)
        }
        tw.Flush(); return nil
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
