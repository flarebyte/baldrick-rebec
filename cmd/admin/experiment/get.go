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
    flagExpGetID string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get an experiment by id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagExpGetID) == "" { return errors.New("--id is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        e, err := pgdao.GetExperimentByID(ctx, db, flagExpGetID)
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "experiment id=%s conversation_id=%s\n", e.ID, e.ConversationID)
        // JSON
        out := map[string]any{"id": e.ID, "conversation_id": e.ConversationID}
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ExperimentCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagExpGetID, "id", "", "Experiment UUID (required)")
}
