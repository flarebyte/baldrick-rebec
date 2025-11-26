package workflow

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "encoding/json"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagWFName  string
    flagWFTitle string
    flagWFDesc  string
    flagWFNotes string
    flagWFRole  string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a workflow (by name)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagWFName) == "" {
            return errors.New("--name is required")
        }
        if strings.TrimSpace(flagWFTitle) == "" {
            return errors.New("--title is required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        w := &pgdao.Workflow{
            Name:  flagWFName,
            Title: flagWFTitle,
        }
        if strings.TrimSpace(flagWFRole) != "" { w.RoleName = strings.TrimSpace(flagWFRole) }
        if flagWFDesc != "" { w.Description = sql.NullString{String: flagWFDesc, Valid: true} }
        if flagWFNotes != "" { w.Notes = sql.NullString{String: flagWFNotes, Valid: true} }
        if err := pgdao.UpsertWorkflow(ctx, db, w); err != nil { return err }
        // Human-friendly one-liner to stderr
        fmt.Fprintf(os.Stderr, "workflow upserted name=%q title=%q\n", w.Name, w.Title)
        // AI/automation friendly JSON to stdout
        out := map[string]any{
            "status": "upserted",
            "name":   w.Name,
            "title":  w.Title,
        }
        if w.Created.Valid { out["created"] = w.Created.Time.Format(time.RFC3339Nano) }
        if w.Updated.Valid { out["updated"] = w.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        if err := enc.Encode(out); err != nil { return err }
        return nil
    },
}

func init() {
    WorkflowCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagWFName, "name", "", "Workflow unique name (required)")
    setCmd.Flags().StringVar(&flagWFTitle, "title", "", "Human-readable title (required)")
    setCmd.Flags().StringVar(&flagWFDesc, "description", "", "Plain text description")
    setCmd.Flags().StringVar(&flagWFNotes, "notes", "", "Markdown-formatted notes")
    setCmd.Flags().StringVar(&flagWFRole, "role", "", "Role name (optional; defaults to 'user')")
}

// no extras
