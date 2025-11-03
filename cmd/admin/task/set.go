package task

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
    flagTaskWF   string
    flagTaskCmd  string
    flagTaskVar  string
    flagTaskVer  string

    flagTaskTitle string
    flagTaskDesc  string
    flagTaskMotiv string
    flagTaskNotes string
    flagTaskShell string
    flagTaskRun   string
    flagTaskTimeout string
    flagTaskTags    []string
    flagTaskLevel   string
    flagTaskToolWS  string
)

var setCmd = &cobra.Command{
    Use:   "set",
    Short: "Create or update a task (by workflow,name,version)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagTaskWF) == "" || strings.TrimSpace(flagTaskCmd) == "" || strings.TrimSpace(flagTaskVer) == "" {
            return errors.New("--workflow, --command and --version are required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        t := &pgdao.Task{WorkflowID: flagTaskWF, Command: flagTaskCmd, Variant: flagTaskVar, Version: flagTaskVer}
        if flagTaskTitle != "" { t.Title = sql.NullString{String: flagTaskTitle, Valid: true} }
        if flagTaskDesc  != "" { t.Description = sql.NullString{String: flagTaskDesc, Valid: true} }
        if flagTaskMotiv != "" { t.Motivation = sql.NullString{String: flagTaskMotiv, Valid: true} }
        if flagTaskNotes != "" { t.Notes = sql.NullString{String: flagTaskNotes, Valid: true} }
        if flagTaskShell != "" { t.Shell = sql.NullString{String: flagTaskShell, Valid: true} }
        if flagTaskRun   != "" { t.Run   = sql.NullString{String: flagTaskRun, Valid: true} }
        if flagTaskTimeout != "" { t.Timeout = sql.NullString{String: flagTaskTimeout, Valid: true} }
        if len(flagTaskTags) > 0 { t.Tags = parseTags(flagTaskTags) }
        if strings.TrimSpace(flagTaskToolWS) != "" { t.ToolWorkspaceID = sql.NullString{String: strings.TrimSpace(flagTaskToolWS), Valid: true} }
        if flagTaskLevel != "" { t.Level = sql.NullString{String: flagTaskLevel, Valid: true} }
        if err := pgdao.UpsertTask(ctx, db, t); err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "task upserted workflow=%q command=%q variant=%q version=%q id=%d\n", t.WorkflowID, t.Command, t.Variant, t.Version, t.ID)
        // JSON
        out := map[string]any{
            "status":"upserted",
            "id": t.ID,
            "workflow": t.WorkflowID,
            "command": t.Command,
            "variant": t.Variant,
            "version": t.Version,
        }
        if t.Created.Valid { out["created"] = t.Created.Time.Format(time.RFC3339Nano) }
        if t.ToolWorkspaceID.Valid { out["tool_workspace_id"] = t.ToolWorkspaceID.String }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    TaskCmd.AddCommand(setCmd)
    setCmd.Flags().StringVar(&flagTaskWF, "workflow", "", "Workflow name (required)")
    setCmd.Flags().StringVar(&flagTaskCmd, "command", "", "Task command (e.g., unit, lint) (required)")
    setCmd.Flags().StringVar(&flagTaskVar, "variant", "", "Task variant (e.g., go, typescript/v5)")
    setCmd.Flags().StringVar(&flagTaskVer, "version", "", "Semver version (required)")
    setCmd.Flags().StringVar(&flagTaskTitle, "title", "", "Human-readable title")
    setCmd.Flags().StringVar(&flagTaskDesc, "description", "", "Plain text description")
    setCmd.Flags().StringVar(&flagTaskMotiv, "motivation", "", "Purpose or context")
    setCmd.Flags().StringVar(&flagTaskNotes, "notes", "", "Markdown notes")
    setCmd.Flags().StringVar(&flagTaskShell, "shell", "", "Shell environment (bash, python)")
    setCmd.Flags().StringVar(&flagTaskRun, "run", "", "Command to execute")
    setCmd.Flags().StringVar(&flagTaskTimeout, "timeout", "", "Text interval, e.g., '5 minutes'")
    setCmd.Flags().StringSliceVar(&flagTaskTags, "tags", nil, "Tags as key=value pairs (repeat or comma-separated). Plain values mapped to true")
    setCmd.Flags().StringVar(&flagTaskLevel, "level", "", "Level: h1..h6")
    setCmd.Flags().StringVar(&flagTaskToolWS, "tool-workspace", "", "Optional workspace UUID used as tooling workspace for the task")
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
