package script

import (
    "bufio"
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagScrID    string
    flagScrRole  string
    flagScrTitle string
    flagScrDesc  string
    flagScrMotiv string
    flagScrNotes string
    flagScrTags  []string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a script (reads stdin for script content)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagScrRole) == "" { return errors.New("--role is required") }
        if strings.TrimSpace(flagScrTitle) == "" { return errors.New("--title is required") }

        // Read stdin script body if piped
        var body []byte
        if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode() & os.ModeCharDevice) == 0 {
            b := &strings.Builder{}
            r := bufio.NewReader(os.Stdin)
            for {
                chunk, err := r.ReadString('\n')
                b.WriteString(chunk)
                if err != nil {
                    if err == io.EOF { break }
                    return fmt.Errorf("read stdin: %w", err)
                }
            }
            body = []byte(b.String())
        }
        if len(body) == 0 { return errors.New("no script content on stdin") }

        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        // Ensure content exists and get its hex id (no role scoping on content)
        cid, err := pgdao.InsertScriptContent(ctx, db, string(body))
        if err != nil { return err }

        // Upsert script
        s := &pgdao.Script{
            ID: flagScrID, Title: flagScrTitle, RoleName: flagScrRole, ScriptContentID: cid,
        }
        if flagScrDesc != "" { s.Description = sql.NullString{String: flagScrDesc, Valid: true} }
        if flagScrMotiv != "" { s.Motivation = sql.NullString{String: flagScrMotiv, Valid: true} }
        if flagScrNotes != "" { s.Notes = sql.NullString{String: flagScrNotes, Valid: true} }
        if len(flagScrTags) > 0 { s.Tags = parseTags(flagScrTags) }
        if err := pgdao.UpsertScript(ctx, db, s); err != nil { return err }

        // stderr summary
        fmt.Fprintf(os.Stderr, "script upserted id=%s title=%q role=%q\n", s.ID, s.Title, s.RoleName)
        // stdout JSON
        out := map[string]any{
            "status": "upserted",
            "id":     s.ID,
            "title":  s.Title,
            "role":   s.RoleName,
            "content_id": cid,
        }
        if s.Created.Valid { out["created"] = s.Created.Time.Format(time.RFC3339Nano) }
        if s.Updated.Valid { out["updated"] = s.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ScriptCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagScrID, "id", "", "Script UUID (optional; when omitted, a new id is generated)")
    setCmd.Flags().StringVar(&flagScrRole, "role", "", "Role name (required)")
    setCmd.Flags().StringVar(&flagScrTitle, "title", "", "Title (required)")
    setCmd.Flags().StringVar(&flagScrDesc, "description", "", "Plain text description")
    setCmd.Flags().StringVar(&flagScrMotiv, "motivation", "", "Motivation or intent")
    setCmd.Flags().StringVar(&flagScrNotes, "notes", "", "Markdown-formatted notes")
    setCmd.Flags().StringSliceVar(&flagScrTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
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
