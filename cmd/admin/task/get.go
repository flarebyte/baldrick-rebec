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
    flagTaskGetName string
    flagTaskGetVer  string
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
            if strings.TrimSpace(flagTaskGetWF)=="" || strings.TrimSpace(flagTaskGetName)=="" || strings.TrimSpace(flagTaskGetVer)=="" {
                return errors.New("provide --id or all of --workflow, --name, --version")
            }
            t, err = pgdao.GetTaskByKey(ctx, db, flagTaskGetWF, flagTaskGetName, flagTaskGetVer)
        }
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "task id=%d workflow=%q name=%q version=%q\n", t.ID, t.WorkflowID, t.Name, t.Version)
        // JSON
        out := map[string]any{
            "id": t.ID,
            "workflow": t.WorkflowID,
            "name": t.Name,
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
    getCmd.Flags().StringVar(&flagTaskGetWF, "workflow", "", "Workflow name (with --name and --version)")
    getCmd.Flags().StringVar(&flagTaskGetName, "name", "", "Task name (with --workflow and --version)")
    getCmd.Flags().StringVar(&flagTaskGetVer, "version", "", "Task version (with --workflow and --name)")
}
