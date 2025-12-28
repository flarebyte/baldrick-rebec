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
	flagTaskLatestVariant string
	flagTaskLatestFromID  string
)

var latestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Find the latest task (by variant or from a given task id)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTaskLatestVariant) == "" && strings.TrimSpace(flagTaskLatestFromID) == "" {
			return errors.New("provide --variant or --from-id")
		}
		cfg, err := cfgpkg.Load()
		if err != nil {
			return fmt.Errorf("cmd=task latest params={variant:%s,from-id:%s}: %w", flagTaskLatestVariant, flagTaskLatestFromID, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		var id string
		if strings.TrimSpace(flagTaskLatestVariant) != "" {
			id, err = pgdao.FindLatestTaskIDByVariant(ctx, db, flagTaskLatestVariant)
		} else {
			id, err = pgdao.FindLatestFrom(ctx, db, flagTaskLatestFromID)
		}
		if err != nil {
			return err
		}
		// If graph lookup didn't find anything, reasonable fallbacks:
		if strings.TrimSpace(id) == "" {
			if strings.TrimSpace(flagTaskLatestFromID) != "" {
				// No replacements found or graph unavailable; return the provided id
				id = flagTaskLatestFromID
			} else if strings.TrimSpace(flagTaskLatestVariant) != "" {
				// With unique variant semantics, latest == the single task for that variant
				t, err := pgdao.GetTaskByVariant(ctx, db, flagTaskLatestVariant)
				if err == nil {
					id = t.ID
				}
			}
		}
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("cmd=task latest no result params={variant:%s,from-id:%s}", flagTaskLatestVariant, flagTaskLatestFromID)
		}
		t, err := pgdao.GetTaskByID(ctx, db, id)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "latest task id=%s variant=%q command=%q\n", t.ID, t.Variant, t.Command)
		out := map[string]any{"id": t.ID, "variant": t.Variant, "command": t.Command}
		if t.Title.Valid {
			out["title"] = t.Title.String
		}
		if t.Created.Valid {
			out["created"] = t.Created.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	TaskCmd.AddCommand(latestCmd)
	latestCmd.Flags().StringVar(&flagTaskLatestVariant, "variant", "", "Task variant to search (e.g., unit/go)")
	latestCmd.Flags().StringVar(&flagTaskLatestFromID, "from-id", "", "Find latest reachable from this task UUID")
}
