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
    "github.com/spf13/cobra"
)

var (
    flagMsgGetID string
    flagMsgExpand bool
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a message by id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagMsgGetID) == "" { return errors.New("--id is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        m, err := pgdao.GetMessageEventByID(ctx, db, flagMsgGetID)
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "message id=%s status=%q\n", m.ID, m.Status)
        // Fetch content for hash and optional expansion
        content, err := pgdao.GetContent(ctx, db, m.ContentID)
        if err != nil { return err }
        hash := pgdao.HashTextSHA256(content.TextContent)
        // JSON
        out := map[string]any{
            "id": m.ID,
            "content_id": m.ContentID,
            "content_id_hash": hash,
            "status": m.Status,
        }
        if m.FromTaskID.Valid { out["from_task_id"] = m.FromTaskID.String }
        if m.ExperimentID.Valid { out["experiment_id"] = m.ExperimentID.String }
        if m.ErrorMessage.Valid { out["error_message"] = m.ErrorMessage.String }
        if len(m.Tags) > 0 { out["tags"] = m.Tags }
        out["created"] = m.Created.Format(time.RFC3339Nano)
        // meta removed
        if flagMsgExpand {
            out["text_content"] = content.TextContent
            out["is_json"] = (len(content.JSONContent) > 0)
        }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    MessageCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagMsgGetID, "id", "", "Message UUID (required)")
    getCmd.Flags().BoolVar(&flagMsgExpand, "expand", false, "Include text_content and is_json in output")
}
