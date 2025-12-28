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
	flagTSAddTaskID2   string
	flagTSAddScriptID2 string
	flagTSAddName2     string
	flagTSAddAlias2    string
)

// ScriptAddCmd attaches a script to a task (same as `task script add` but local to this package).
var ScriptAddCmd = &cobra.Command{
	Use:   "script-add",
	Short: "Attach an existing script to a task with a logical name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTSAddTaskID2) == "" {
			return errors.New("--task is required")
		}
		if strings.TrimSpace(flagTSAddScriptID2) == "" {
			return errors.New("--script is required")
		}
		if strings.TrimSpace(flagTSAddName2) == "" {
			return errors.New("--name is required")
		}
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		var alias sql.NullString
		if strings.TrimSpace(flagTSAddAlias2) != "" {
			alias = sql.NullString{String: flagTSAddAlias2, Valid: true}
		}
		ts, err := pgdao.AddTaskScript(ctx, db, flagTSAddTaskID2, flagTSAddScriptID2, flagTSAddName2, alias)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "attached script %s to task %s as name=%q alias=%q (id=%s)\n", ts.ScriptID, ts.TaskID, ts.Name, valueOr(flagTSAddAlias2, ""), ts.ID)
		fmt.Fprintln(os.Stdout, ts.ID)
		return nil
	},
}

func init() {
	TaskCmd.AddCommand(ScriptAddCmd)
	ScriptAddCmd.Flags().StringVar(&flagTSAddTaskID2, "task", "", "Task UUID (required)")
	ScriptAddCmd.Flags().StringVar(&flagTSAddScriptID2, "script", "", "Script UUID (required)")
	ScriptAddCmd.Flags().StringVar(&flagTSAddName2, "name", "", "Logical name, e.g., 'run' (required)")
	ScriptAddCmd.Flags().StringVar(&flagTSAddAlias2, "alias", "", "Optional alternate lookup name")
}
