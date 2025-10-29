package experiment

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
    flagExpConversation string
)

var createCmd = &cobra.Command{
    Use:   "create",
    Short: "Create a new experiment linked to a conversation",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagExpConversation) == "" { return errors.New("--conversation is required") }
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
        fmt.Fprintf(os.Stderr, "experiment created id=%s conversation_id=%s\n", e.ID, e.ConversationID)
        // JSON
        out := map[string]any{"status":"created","id":e.ID,"conversation_id":e.ConversationID}
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ExperimentCmd.AddCommand(createCmd)
    createCmd.Flags().StringVar(&flagExpConversation, "conversation", "", "Conversation UUID (required)")
}
