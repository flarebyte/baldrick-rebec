package message

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "os"
    "strings"
    "encoding/json"
    "context"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

// Flags
var (
    flagConversation string
    flagAttempt      string
    flagProfile      string

    flagTitle       string
    flagLevel       string
    flagFrom        string
    flagTo          []string
    flagTags        []string
    flagDescription string
    flagGoal        string
    flagTimeout     string
)

var sendCmd = &cobra.Command{
    Use:   "send",
    Short: "Send a structured message (logs stdin, echoes to stdout)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, _ := cfgpkg.Load()
        // Validate mandatory params
        if strings.TrimSpace(flagConversation) == "" {
            return errors.New("missing required flag: --conversation")
        }
        if strings.TrimSpace(flagAttempt) == "" {
            return errors.New("missing required flag: --attempt")
        }
        if strings.TrimSpace(flagProfile) == "" {
            return errors.New("missing required flag: --profile")
        }

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
                    if errors.Is(err, io.EOF) {
                        break
                    }
                    return fmt.Errorf("read stdin: %w", err)
                }
            }
            stdinData = []byte(b.String())
        }

        // Log parsed parameters to stderr, to avoid polluting stdout pipeline
        fmt.Fprintf(os.Stderr, "rbc admin message send: conversation=%q attempt=%q profile=%q title=%q level=%q from=%q to=%q tags=%q timeout=%q\n",
            flagConversation, flagAttempt, flagProfile, flagTitle, flagLevel, flagFrom,
            strings.Join(flagTo, ","), strings.Join(flagTags, ","), flagTimeout,
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
        // If pg-only, persist content + event to Postgres
        if cfg.Features.PGOnly {
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
                "from":  flagFrom,
                "to":    flagTo,
                "tags":  flagTags,
                "description": flagDescription,
                "goal":  flagGoal,
            }
            metaJSON, _ := json.Marshal(meta)
            id, err := pgdao.PutMessageContent(ctx, db, string(stdinData), flagProfile, "", metaJSON)
            if err != nil { return err }
            // Insert event referencing content
            ev := &pgdao.MessageEvent{
                ContentID: id,
                ConversationID: flagConversation,
                AttemptID: flagAttempt,
                ProfileName: flagProfile,
                Source: "cli",
                Status: "ingested",
                Attempt: 1,
                Recipients: flagTo,
                Tags: flagTags,
            }
            if _, err := pgdao.InsertMessageEvent(ctx, db, ev); err != nil { return err }
            fmt.Fprintf(os.Stderr, "pg-only: stored content id=%s and event\n", id)
        }
        return nil
    },
}

func init() {
    // Mandatory
    sendCmd.Flags().StringVar(&flagConversation, "conversation", "", "Conversation identifier")
    sendCmd.Flags().StringVar(&flagAttempt, "attempt", "", "Attempt identifier")
    sendCmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name for message metadata")

    // Optional
    sendCmd.Flags().StringVar(&flagTitle, "title", "", "Short label for the message")
    sendCmd.Flags().StringVar(&flagLevel, "level", "", "Hierarchical depth (e.g., h1, h2, h3)")
    sendCmd.Flags().StringVar(&flagFrom, "from", "", "Sender identifier (agent or user)")
    sendCmd.Flags().StringSliceVar(&flagTo, "to", nil, "Target recipients (comma-separated or repeated)")
    sendCmd.Flags().StringSliceVar(&flagTags, "tags", nil, "Tags (comma-separated or repeated)")
    sendCmd.Flags().StringVar(&flagDescription, "description", "", "Longer explanation or context")
    sendCmd.Flags().StringVar(&flagGoal, "goal", "", "Intended outcome of the message")
    sendCmd.Flags().StringVar(&flagTimeout, "timeout", "", "Max allowed duration for execution or response")
}
