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

var (
    flagTaskGetID  int64
    flagTaskGetWF  string
    flagTaskGetCmd string
    flagTaskGetVar string
    flagTaskGetVer string
)

var getCmd = &cobra.Command{
    Use:   "get",
    Short: "Get a task by id or by (workflow,name,version)",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        var t *pgdao.Task
        if flagTaskGetID > 0 {
            t, err = pgdao.GetTaskByID(ctx, db, flagTaskGetID)
        } else {
            if strings.TrimSpace(flagTaskGetVer)=="" || (strings.TrimSpace(flagTaskGetVar)=="" && strings.TrimSpace(flagTaskGetCmd)=="") {
                return errors.New("provide --id or --version and either --variant (full selector) or --command (used as selector if no variant provided)")
            }
            selector := flagTaskGetVar
            if selector == "" { selector = flagTaskGetCmd }
            t, err = pgdao.GetTaskByKey(ctx, db, selector, flagTaskGetVer)
        }
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "task id=%d workflow=%q command=%q variant=%q version=%q\n", t.ID, t.WorkflowID, t.Command, t.Variant, t.Version)
        // JSON
        out := map[string]any{
            "id": t.ID,
            "workflow": t.WorkflowID,
            "command": t.Command,
            "variant": t.Variant,
            "version": t.Version,
        }
        if t.Created.Valid { out["created"] = t.Created.Time.Format(time.RFC3339Nano) }
        if t.Title.Valid { out["title"] = t.Title.String }
        if t.Description.Valid { out["description"] = t.Description.String }
        if t.Motivation.Valid { out["motivation"] = t.Motivation.String }
        if t.Notes.Valid { out["notes"] = t.Notes.String }
        if t.Shell.Valid { out["shell"] = t.Shell.String }
        if t.Run.Valid { out["run"] = t.Run.String }
        if t.Timeout.Valid { out["timeout"] = t.Timeout.String }
        if len(t.Tags) > 0 { out["tags"] = t.Tags }
        if t.Level.Valid { out["level"] = t.Level.String }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    TaskCmd.AddCommand(getCmd)
    getCmd.Flags().Int64Var(&flagTaskGetID, "id", 0, "Task numeric id")
    getCmd.Flags().StringVar(&flagTaskGetWF, "workflow", "", "Workflow name (with --command and --version)")
    getCmd.Flags().StringVar(&flagTaskGetCmd, "command", "", "Task command (with --workflow and --version)")
    getCmd.Flags().StringVar(&flagTaskGetVar, "variant", "", "Task variant (optional; default empty)")
    getCmd.Flags().StringVar(&flagTaskGetVer, "version", "", "Task version (with --workflow and --command)")
}
