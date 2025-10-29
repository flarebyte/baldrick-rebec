package conversation

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
    flagConvGetID string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a conversation by id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagConvGetID) == "" { return errors.New("--id is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        c, err := pgdao.GetConversationByID(ctx, db, flagConvGetID)
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "conversation id=%d title=%q\n", c.ID, c.Title)
        // JSON
        out := map[string]any{"id": c.ID, "title": c.Title}
        if c.Project.Valid { out["project"] = c.Project.String }
        if c.Description.Valid { out["description"] = c.Description.String }
        if len(c.Tags) > 0 { out["tags"] = c.Tags }
        if c.Notes.Valid { out["notes"] = c.Notes.String }
        if c.Created.Valid { out["created"] = c.Created.Time.Format(time.RFC3339Nano) }
        if c.Updated.Valid { out["updated"] = c.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    ConversationCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagConvGetID, "id", "", "Conversation UUID (required)")
}
