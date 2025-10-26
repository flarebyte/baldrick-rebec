package task

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagTaskFindWF  string
    flagTaskFindVar string
    flagTaskFindVer string
)

var findCmd = &cobra.Command{
    Use:   "find",
    Short: "Find a single task by (workflow, variant, version)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagTaskFindWF == "" || flagTaskFindVar == "" || flagTaskFindVer == "" {
            return errors.New("--workflow, --variant and --version are required")
        }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()
        t, err := pgdao.GetTaskByKey(ctx, db, flagTaskFindWF, flagTaskFindVar, flagTaskFindVer)
        if err != nil { return err }
        // Human
        fmt.Fprintf(os.Stderr, "task id=%d workflow=%q command=%q variant=%q version=%q\n", t.ID, t.WorkflowID, t.Command, t.Variant, t.Version)
        // JSON
        out := map[string]any{"id": t.ID, "workflow": t.WorkflowID, "command": t.Command, "variant": t.Variant, "version": t.Version}
        if t.Created.Valid { out["created"] = t.Created.Time.Format(time.RFC3339Nano) }
        enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  "); return enc.Encode(out)
    },
}

func init() {
    TaskCmd.AddCommand(findCmd)
    findCmd.Flags().StringVar(&flagTaskFindWF, "workflow", "", "Workflow name (required)")
    findCmd.Flags().StringVar(&flagTaskFindVar, "variant", "", "Task selector, e.g., unit/go (required)")
    findCmd.Flags().StringVar(&flagTaskFindVer, "version", "", "Semver version (required)")
}

