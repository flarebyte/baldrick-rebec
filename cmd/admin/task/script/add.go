package task

import (
    "context"
    "database/sql"
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
    flagTSAddTaskID   string
    flagTSAddScriptID string
    flagTSAddName     string
    flagTSAddAlias    string
)

var tsAddCmd = &cobra.Command{
    Use:   "add",
    Short: "Attach an existing script to a task with a logical name",
    RunE: func(cmd *cobra.Command, args []string) error {
        if strings.TrimSpace(flagTSAddTaskID) == "" { return errors.New("--task is required") }
        if strings.TrimSpace(flagTSAddScriptID) == "" { return errors.New("--script is required") }
        if strings.TrimSpace(flagTSAddName) == "" { return errors.New("--name is required") }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        var alias sql.NullString
        if strings.TrimSpace(flagTSAddAlias) != "" { alias = sql.NullString{String: flagTSAddAlias, Valid: true} }
        ts, err := pgdao.AddTaskScript(ctx, db, flagTSAddTaskID, flagTSAddScriptID, flagTSAddName, alias)
        if err != nil { return err }
        fmt.Fprintf(os.Stderr, "attached script %s to task %s as name=%q alias=%q (id=%s)\n", ts.ScriptID, ts.TaskID, ts.Name, valueOr(flagTSAddAlias, ""), ts.ID)
        fmt.Fprintf(os.Stdout, "%s\n", ts.ID)
        return nil
    },
}

func init() {
    ScriptCmd.AddCommand(tsAddCmd)
    tsAddCmd.Flags().StringVar(&flagTSAddTaskID, "task", "", "Task UUID (required)")
    tsAddCmd.Flags().StringVar(&flagTSAddScriptID, "script", "", "Script UUID (required)")
    tsAddCmd.Flags().StringVar(&flagTSAddName, "name", "", "Logical name, e.g., 'run' (required)")
    tsAddCmd.Flags().StringVar(&flagTSAddAlias, "alias", "", "Optional alternate lookup name")
}
