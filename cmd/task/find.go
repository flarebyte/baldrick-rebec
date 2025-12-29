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
	flagTaskFindVar          string
	flagTaskFindActiveOnly   bool
	flagTaskFindArchivedOnly bool
)

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find a single task by variant",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagTaskFindVar == "" {
			return errors.New("--variant is required")
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
		if flagTaskFindActiveOnly && flagTaskFindArchivedOnly {
			return errors.New("--active-only and --archived-only are mutually exclusive")
		}
		t, err := pgdao.GetTaskByVariant(ctx, db, flagTaskFindVar)
		if err != nil {
			return err
		}
		if flagTaskFindActiveOnly && t.Archived {
			return fmt.Errorf("task %q is archived; use --archived-only or omit filters", t.Variant)
		}
		if flagTaskFindArchivedOnly && !t.Archived {
			return fmt.Errorf("task %q is active; use --active-only or omit filters", t.Variant)
		}
		// Human
		fmt.Fprintf(os.Stderr, "task id=%s workflow=%q command=%q variant=%q\n", t.ID, t.WorkflowID, t.Command, t.Variant)
		// JSON
		out := map[string]any{"id": t.ID, "workflow": t.WorkflowID, "command": t.Command, "variant": t.Variant}
		if t.Created.Valid {
			out["created"] = t.Created.Time.Format(time.RFC3339Nano)
		}
		if t.Archived {
			out["archived"] = true
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	TaskCmd.AddCommand(findCmd)
	findCmd.Flags().StringVar(&flagTaskFindVar, "variant", "", "Task selector, e.g., unit/go (required)")
	findCmd.Flags().BoolVar(&flagTaskFindActiveOnly, "active-only", false, "Require the task to be non-archived")
	findCmd.Flags().BoolVar(&flagTaskFindArchivedOnly, "archived-only", false, "Require the task to be archived")
}
