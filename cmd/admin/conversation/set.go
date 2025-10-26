package conversation

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
    flagConvID    int64
    flagConvTitle string
    flagConvDesc  string
    flagConvNotes string
    flagConvProj  string
    flagConvTags  []string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a conversation (by id)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagConvTitle) == "" { return errors.New("--title is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        conv := &pgdao.Conversation{ ID: flagConvID, Title: flagConvTitle }
        if flagConvDesc != "" { conv.Description = sql.NullString{String: flagConvDesc, Valid: true} }
        if flagConvNotes != "" { conv.Notes = sql.NullString{String: flagConvNotes, Valid: true} }
        if flagConvProj != "" { conv.Project = sql.NullString{String: flagConvProj, Valid: true} }
        if len(flagConvTags) > 0 { conv.Tags = flagConvTags }
        if err := pgdao.UpsertConversation(ctx, db, conv); err != nil { return err }
        // Human line
        fmt.Fprintf(os.Stderr, "conversation upserted id=%d title=%q\n", conv.ID, conv.Title)
        // JSON
        out := map[string]any{
            "status": "upserted",
            "id": conv.ID,
            "title": conv.Title,
        }
        if conv.Project.Valid { out["project"] = conv.Project.String }
        if conv.Created.Valid { out["created"] = conv.Created.Time.Format(time.RFC3339Nano) }
        if conv.Updated.Valid { out["updated"] = conv.Updated.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ConversationCmd.AddCommand(setCmd)
    setCmd.Flags().Int64Var(&flagConvID, "id", 0, "Conversation id (optional; when omitted, a new id is generated)")
    setCmd.Flags().StringVar(&flagConvTitle, "title", "", "Title (required)")
    setCmd.Flags().StringVar(&flagConvDesc, "description", "", "Plain text description")
    setCmd.Flags().StringVar(&flagConvNotes, "notes", "", "Markdown notes")
    setCmd.Flags().StringVar(&flagConvProj, "project", "", "Project name (e.g. GitHub repo)")
    setCmd.Flags().StringSliceVar(&flagConvTags, "tags", nil, "Tags")
}
