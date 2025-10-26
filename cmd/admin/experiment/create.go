package experiment

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
    flagExpConversation int64
)

var createCmd = &cobra.Command{
    Use:   "create",
    Short: "Create a new experiment linked to a conversation",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagExpConversation <= 0 {
            return errors.New("--conversation is required and must be > 0")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        e, err := pgdao.CreateExperiment(ctx, db, flagExpConversation)
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "experiment created id=%d conversation_id=%d\n", e.ID, e.ConversationID)
        // JSON
        out := map[string]any{"status":"created","id":e.ID,"conversation_id":e.ConversationID}
        if e.Created.Valid { out["created"] = e.Created.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ExperimentCmd.AddCommand(createCmd)
    createCmd.Flags().Int64Var(&flagExpConversation, "conversation", 0, "Conversation id (required)")
}

