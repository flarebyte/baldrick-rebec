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

// yamlUnmarshal is a thin wrapper around yaml.v3 if available.
// We provide it via a buildable shim in this file.
func yamlUnmarshal(b []byte, v any) error {
    // Use a light dependency via gopkg.in/yaml.v3
    type Y = any
    return yamlUnmarshalImpl(b, v)
}

// Flags
var (
    flagTitle       string
    flagLevel       string
    flagFrom        string
    flagTags        []string
    flagDescription string
    flagGoal        string
    flagExperiment  string
    flagFormat      string
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
            // Meta moved into content JSON when using --format (compose and store in content json if parsed)
            meta := map[string]interface{}{
                "title": flagTitle,
                "level": flagLevel,
                "executor":  flagFrom,
                "tags":  parseTags(flagTags),
                "description": flagDescription,
                "goal":  flagGoal,
            }
            // Handle optional structured content
            var parsed []byte
            var parseErr error
            switch strings.ToLower(strings.TrimSpace(flagFormat)) {
            case "json":
                var v any
                if err := json.Unmarshal(stdinData, &v); err != nil {
                    parseErr = fmt.Errorf("invalid json: %w", err)
                } else {
                    // attach meta next to parsed payload
                    env := map[string]any{"content": v, "meta": meta}
                    parsed, _ = json.Marshal(env)
                }
            case "yaml":
                // Lazy import to avoid hard dependency if not used
                type yamlAny = any
                var ya yamlAny
                if err := yamlUnmarshal(stdinData, &ya); err != nil {
                    parseErr = fmt.Errorf("invalid yaml: %w", err)
                } else {
                    env := map[string]any{"content": ya, "meta": meta}
                    parsed, _ = json.Marshal(env)
                }
            case "":
                // no parsing
            default:
                return fmt.Errorf("unsupported --format value: %s", flagFormat)
            }
            cid, insErr := pgdao.InsertContent(ctx, db, string(stdinData), parsed)
            if insErr != nil { return insErr }
            // Insert event referencing content
            ev := &pgdao.MessageEvent{ ContentID: cid, Status: "ingested", Tags: parseTags(flagTags) }
            // Map optional executor and experiment
            if strings.TrimSpace(flagFrom) != "" { ev.Executor = sql.NullString{String: flagFrom, Valid: true} }
            if strings.TrimSpace(flagExperiment) != "" { ev.ExperimentID = sql.NullString{String: flagExperiment, Valid: true} }
            if _, err := pgdao.InsertMessageEvent(ctx, db, ev); err != nil { return err }
            fmt.Fprintf(os.Stderr, "stored content id=%d and message row\n", cid)
            // Return parse error if any (after storing content and message)
            if parseErr != nil { return parseErr }
        return nil
    },
}

func init() {
    // Optional
    setCmd.Flags().StringVar(&flagTitle, "title", "", "Short label for the message")
    setCmd.Flags().StringVar(&flagLevel, "level", "", "Hierarchical depth (e.g., h1, h2, h3)")
    setCmd.Flags().StringVar(&flagFrom, "executor", "", "Executor identifier (actor id)")
    setCmd.Flags().StringSliceVar(&flagTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
    setCmd.Flags().StringVar(&flagDescription, "description", "", "Longer explanation or context")
    setCmd.Flags().StringVar(&flagGoal, "goal", "", "Intended outcome of the message")
    setCmd.Flags().StringVar(&flagExperiment, "experiment", "", "Experiment UUID to link this message to")
    setCmd.Flags().StringVar(&flagFormat, "format", "", "Optional: interpret stdin as json or yaml and save parsed JSON alongside text")
}

// parseTags converts k=v pairs (or bare keys) into a map.
func parseTags(items []string) map[string]any {
    if len(items) == 0 { return nil }
    out := map[string]any{}
    for _, raw := range items {
        if raw == "" { continue }
        parts := strings.Split(raw, ",")
        for _, p := range parts {
            p = strings.TrimSpace(p)
            if p == "" { continue }
            if eq := strings.IndexByte(p, '='); eq > 0 {
                k := strings.TrimSpace(p[:eq])
                v := strings.TrimSpace(p[eq+1:])
                if k != "" { out[k] = v }
            } else {
                out[p] = true
            }
        }
    }
    return out
}
