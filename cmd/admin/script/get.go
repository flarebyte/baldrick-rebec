package script

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
    flagScrGetID string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a script by id",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagScrGetID) == "" { return errors.New("--id is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        s, err := pgdao.GetScriptByID(ctx, db, flagScrGetID)
        if err != nil { return err }
        // stderr line
        fmt.Fprintf(os.Stderr, "script id=%s title=%q role=%q\n", s.ID, s.Title, s.RoleName)
        // stdout JSON
        out := map[string]any{
            "id": s.ID,
            "title": s.Title,
            "role": s.RoleName,
            "content_id": s.ScriptContentID,
        }
        if s.ComplexName.Name != "" || s.ComplexName.Variant != "" {
            out["name"] = s.ComplexName.Name
            out["variant"] = s.ComplexName.Variant
        }
        if s.Archived { out["archived"] = true }
        if s.Created.Valid { out["created"] = s.Created.Time.Format(time.RFC3339Nano) }
        if s.Updated.Valid { out["updated"] = s.Updated.Time.Format(time.RFC3339Nano) }
        if s.Description.Valid && s.Description.String != "" { out["description"] = s.Description.String }
        if s.Motivation.Valid && s.Motivation.String != "" { out["motivation"] = s.Motivation.String }
        if s.Notes.Valid && s.Notes.String != "" { out["notes"] = s.Notes.String }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    ScriptCmd.AddCommand(getCmd)
    getCmd.Flags().StringVar(&flagScrGetID, "id", "", "Script UUID (required)")
}
