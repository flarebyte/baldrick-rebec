package tag

import (
    "context"
    "database/sql"
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
    flagTagName  string
    flagTagTitle string
    flagTagDesc  string
    flagTagNotes string
    flagTagRole  string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a tag (by name)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagTagName) == "" { return errors.New("--name is required") }
        if strings.TrimSpace(flagTagTitle) == "" { return errors.New("--title is required") }

        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        t := &pgdao.Tag{ Name: flagTagName, Title: flagTagTitle }
        if strings.TrimSpace(flagTagRole) != "" { t.RoleName = strings.TrimSpace(flagTagRole) }
        if flagTagDesc != "" { t.Description = sql.NullString{String: flagTagDesc, Valid: true} }
        if flagTagNotes != "" { t.Notes = sql.NullString{String: flagTagNotes, Valid: true} }
        if err := pgdao.UpsertTag(ctx, db, t); err != nil { return err }

        // stderr summary
        fmt.Fprintf(os.Stderr, "tag upserted name=%q title=%q\n", t.Name, t.Title)
        // stdout JSON
        out := map[string]any{
            "status": "upserted",
            "name":   t.Name,
            "title":  t.Title,
        }
        if t.Created.Valid { out["created"] = t.Created.Time.Format(time.RFC3339Nano) }
        if t.Updated.Valid { out["updated"] = t.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    TagCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagTagName, "name", "", "Tag unique name (required)")
    setCmd.Flags().StringVar(&flagTagTitle, "title", "", "Human-readable title (required)")
    setCmd.Flags().StringVar(&flagTagDesc, "description", "", "Plain text description")
    setCmd.Flags().StringVar(&flagTagNotes, "notes", "", "Markdown-formatted notes")
    setCmd.Flags().StringVar(&flagTagRole, "role", "", "Role name (optional; defaults to 'user')")
}
