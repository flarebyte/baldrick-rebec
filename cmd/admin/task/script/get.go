package task

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

var tsGetCmd = &cobra.Command{
    Use:   "get TASK_ID NAME_OR_ALIAS",
    Short: "Resolve a script for a task by name or alias",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        taskID := strings.TrimSpace(args[0])
        nameOrAlias := strings.TrimSpace(args[1])
        if taskID == "" || nameOrAlias == "" { return errors.New("TASK_ID and NAME_OR_ALIAS are required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        s, err := pgdao.ResolveTaskScript(ctx, db, taskID, nameOrAlias)
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
    ScriptCmd.AddCommand(tsGetCmd)
}
