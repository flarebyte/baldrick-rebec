package message

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagContentGetID int64
)

var contentGetCmd = &cobra.Command{
    Use:   "get",
    Short: "Get message content by content id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagContentGetID <= 0 { return errors.New("--id is required and must be > 0") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        c, err := pgdao.GetContent(ctx, db, flagContentGetID)
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "content id=%d json=%v\n", c.ID, len(c.JSONContent) > 0)
        // JSON
        out := map[string]any{
            "id": c.ID,
            "text_content": c.TextContent,
            "is_json": len(c.JSONContent) > 0,
        }
        if len(c.JSONContent) > 0 {
            var anyJSON any
            _ = json.Unmarshal(c.JSONContent, &anyJSON)
            out["json_content"] = anyJSON
        }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ContentCmd.AddCommand(contentGetCmd)
    contentGetCmd.Flags().Int64Var(&flagContentGetID, "id", 0, "Content id (required)")
}

