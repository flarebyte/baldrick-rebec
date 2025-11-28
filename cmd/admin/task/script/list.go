package task

import (
    "context"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/olekukonko/tablewriter"
    "github.com/spf13/cobra"
)

var tsListCmd = &cobra.Command{
    Use:   "list TASK_ID",
    Short: "List scripts attached to a task",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        taskID := args[0]
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        items, err := pgdao.ListTaskScripts(ctx, db, taskID)
        if err != nil { return err }

        table := tablewriter.NewWriter(os.Stdout)
        table.SetHeader([]string{"ID", "SCRIPT_ID", "NAME", "ALIAS", "CREATED"})
        for _, it := range items {
            created := ""
            if it.CreatedAt.Valid { created = it.CreatedAt.Time.Format(time.RFC3339) }
            alias := ""
            if it.Alias.Valid { alias = it.Alias.String }
            table.Append([]string{it.ID, it.ScriptID, it.Name, alias, created})
        }
        table.Render()
        return nil
    },
}

func init() {
    ScriptCmd.AddCommand(tsListCmd)
}
