package experiment

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
    flagExpListConv   string
    flagExpListLimit  int
    flagExpListOffset int
    flagExpListMax    int
)

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List experiments (optionally filter by conversation)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        effLimit := flagExpListMax
        if effLimit <= 0 { effLimit = flagExpListLimit }
        rows, err := pgdao.ListExperiments(ctx, db, flagExpListConv, effLimit, flagExpListOffset)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "experiments: %d\n", len(rows))
        arr := make([]map[string]any, 0, len(rows))
        for _, e := range rows {
            item := map[string]any{"id": e.ID, "conversation_id": e.ConversationID}
            arr = append(arr, item)
        }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  ")
        return enc.Encode(arr)
    },
}

func init() {
    ExperimentCmd.AddCommand(listCmd)
    listCmd.Flags().StringVar(&flagExpListConv, "conversation", "", "Filter by conversation UUID")
    listCmd.Flags().IntVar(&flagExpListLimit, "limit", 100, "Max rows (deprecated; prefer --max-results)")
    listCmd.Flags().IntVar(&flagExpListOffset, "offset", 0, "Offset for pagination")
    listCmd.Flags().IntVar(&flagExpListMax, "max-results", 20, "Max results to return (default 20)")
}
