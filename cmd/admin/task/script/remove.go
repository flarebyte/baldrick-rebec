package task

import (
	"context"
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
	flagTSRemoveTaskID string
)

var tsRemoveCmd = &cobra.Command{
	Use:   "remove [name-or-alias]",
	Short: "Detach a script from a task by name or alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTSRemoveTaskID) == "" {
			return errors.New("--task is required")
		}
		nameOrAlias := args[0]
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

		n, err := pgdao.RemoveTaskScript(ctx, db, flagTSRemoveTaskID, nameOrAlias)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("no script with name or alias %q on task %s", nameOrAlias, flagTSRemoveTaskID)
		}
		fmt.Fprintf(os.Stderr, "removed %d attachment(s)\n", n)
		fmt.Fprintf(os.Stdout, "%d\n", n)
		return nil
	},
}

func init() {
	ScriptCmd.AddCommand(tsRemoveCmd)
	tsRemoveCmd.Flags().StringVar(&flagTSRemoveTaskID, "task", "", "Task UUID (required)")
}
