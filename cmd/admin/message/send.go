package message

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "strings"
    "encoding/json"
    "context"
    "database/sql"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

// Flags
var (
    flagTitle       string
    flagLevel       string
    flagFrom        string
    flagTags        []string
    flagDescription string
    flagGoal        string
    flagExperiment  int64
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create a message record (reads stdin for content)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, _ := cfgpkg.Load()

        // Read stdin if piped; avoid blocking if attached to TTY
        var stdinData []byte
        if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
            // Data is being piped in
            b := &strings.Builder{}
            reader := bufio.NewReader(os.Stdin)
            for {
                chunk, err := reader.ReadString('\n')
                b.WriteString(chunk)
                if err != nil {
                    if err == io.EOF {
                        break
                    }
                    return fmt.Errorf("read stdin: %w", err)
                }
            }
            stdinData = []byte(b.String())
        }

        // Log parsed parameters to stderr, to avoid polluting stdout pipeline
        fmt.Fprintf(os.Stderr, "rbc admin message set: title=%q level=%q executor=%q tags=%q\n",
            flagTitle, flagLevel, flagFrom, strings.Join(flagTags, ","),
        )
        if flagDescription != "" || flagGoal != "" {
            fmt.Fprintf(os.Stderr, "description=%q goal=%q\n", flagDescription, flagGoal)
        }

        // Echo stdin to stdout so downstream pipe continues to work
        if len(stdinData) > 0 {
            if _, err := os.Stdout.Write(stdinData); err != nil {
                return fmt.Errorf("write stdout: %w", err)
            }
        }
        // Persist content + event to Postgres
            ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
            defer cancel()
            db, err := pgdao.OpenApp(ctx, cfg)
            if err != nil { return err }
            defer db.Close()
            if err := pgdao.EnsureContentSchema(ctx, db); err != nil { return err }
            // Prepare metadata as JSON
            meta := map[string]interface{}{
                "title": flagTitle,
                "level": flagLevel,
                "executor":  flagFrom,
                "tags":  flagTags,
                "description": flagDescription,
                "goal":  flagGoal,
            }
            metaJSON, _ := json.Marshal(meta)
            id, err := pgdao.PutMessageContent(ctx, db, string(stdinData), "", "", metaJSON)
            if err != nil { return err }
            // Insert event referencing content
            ev := &pgdao.MessageEvent{ ContentID: id, Status: "ingested", Tags: flagTags }
            // Map optional executor and experiment
            if strings.TrimSpace(flagFrom) != "" { ev.Executor = sql.NullString{String: flagFrom, Valid: true} }
            if flagExperiment > 0 { ev.ExperimentID = sql.NullInt64{Int64: flagExperiment, Valid: true} }
            if _, err := pgdao.InsertMessageEvent(ctx, db, ev); err != nil { return err }
            fmt.Fprintf(os.Stderr, "stored content id=%s and message row\n", id)
        return nil
    },
}

func init() {
    // Optional
    setCmd.Flags().StringVar(&flagTitle, "title", "", "Short label for the message")
    setCmd.Flags().StringVar(&flagLevel, "level", "", "Hierarchical depth (e.g., h1, h2, h3)")
    setCmd.Flags().StringVar(&flagFrom, "executor", "", "Executor identifier (actor id)")
    setCmd.Flags().StringSliceVar(&flagTags, "tags", nil, "Tags (comma-separated or repeated)")
    setCmd.Flags().StringVar(&flagDescription, "description", "", "Longer explanation or context")
    setCmd.Flags().StringVar(&flagGoal, "goal", "", "Intended outcome of the message")
    setCmd.Flags().Int64Var(&flagExperiment, "experiment", 0, "Experiment id to link this message to")
}
