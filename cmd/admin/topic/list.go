package topic

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
    flagTopicListLimit  int
    flagTopicListOffset int
    flagTopicListOutput string
    flagTopicListRole   string
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List topics for a role (paginated)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagTopicListRole) == "" { return errors.New("--role is required") }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        ts, err := pgdao.ListTopics(ctx, db, flagTopicListRole, flagTopicListLimit, flagTopicListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "topics: %d\n", len(ts))
        out := strings.ToLower(strings.TrimSpace(flagTopicListOutput))
        if out == "json" {
            arr := make([]map[string]any, 0, len(ts))
            for _, t := range ts {
                item := map[string]any{"name": t.Name, "role": t.RoleName, "title": t.Title}
                if t.Created.Valid { item["created"] = t.Created.Time.Format(time.RFC3339Nano) }
                if t.Updated.Valid { item["updated"] = t.Updated.Time.Format(time.RFC3339Nano) }
                arr = append(arr, item)
            }
            enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
        }
        // table default
        tw := tt.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        fmt.Fprintln(tw, "NAME\tTITLE\tUPDATED")
        for _, t := range ts {
            updated := ""; if t.Updated.Valid { updated = t.Updated.Time.Format(time.RFC3339) }
            fmt.Fprintf(tw, "%s\t%s\t%s\n", t.Name, t.Title, updated)
        }
        tw.Flush(); return nil
    },
}

func init() {
    TopicCmd.AddCommand(listCmd)
    listCmd.Flags().IntVar(&flagTopicListLimit, "limit", 100, "Max number of rows")
    listCmd.Flags().IntVar(&flagTopicListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().StringVar(&flagTopicListOutput, "output", "table", "Output format: table or json")
    listCmd.Flags().StringVar(&flagTopicListRole, "role", "", "Role name (required)")
}

