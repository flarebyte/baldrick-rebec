package workflow

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
    flagWFListLimit  int
    flagWFListOffset int
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List workflows (paginated)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        ws, err := pgdao.ListWorkflows(ctx, db, flagWFListLimit, flagWFListOffset)
        if err != nil { return err }
        // Human summary on stderr
        fmt.Fprintf(os.Stderr, "workflows: %d\n", len(ws))
        // JSON array on stdout
        arr := make([]map[string]any, 0, len(ws))
        for _, w := range ws {
            item := map[string]any{
                "name":  w.Name,
                "title": w.Title,
            }
            if w.Created.Valid { item["created"] = w.Created.Time.Format(time.RFC3339Nano) }
            if w.Updated.Valid { item["updated"] = w.Updated.Time.Format(time.RFC3339Nano) }
            if w.Description.Valid && w.Description.String != "" { item["description"] = w.Description.String }
            if w.Notes.Valid && w.Notes.String != "" { item["notes"] = w.Notes.String }
            arr = append(arr, item)
        }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(arr)
    },
}

func init() {
    WorkflowCmd.AddCommand(listCmd)
    listCmd.Flags().IntVar(&flagWFListLimit, "limit", 100, "Max number of rows")
    listCmd.Flags().IntVar(&flagWFListOffset, "offset", 0, "Offset for pagination")
}

