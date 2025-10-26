package conversation

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagConvListProject string
    flagConvListLimit   int
    flagConvListOffset  int
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List conversations (optionally filter by project)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        rows, err := pgdao.ListConversations(ctx, db, flagConvListProject, flagConvListLimit, flagConvListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "conversations: %d\n", len(rows))
        arr := make([]map[string]any, 0, len(rows))
        for _, c := range rows {
            item := map[string]any{"id": c.ID, "title": c.Title}
            if c.Project.Valid { item["project"] = c.Project.String }
            if c.Created.Valid { item["created"] = c.Created.Time.Format(time.RFC3339Nano) }
            if len(c.Tags) > 0 { item["tags"] = c.Tags }
            arr = append(arr, item)
        }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(arr)
    },
}

func init() {
    ConversationCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagConvListProject, "project", "", "Filter by project")
    listCmd.Flags().IntVar(&flagConvListLimit, "limit", 100, "Max rows")
    listCmd.Flags().IntVar(&flagConvListOffset, "offset", 0, "Offset for pagination")
}

