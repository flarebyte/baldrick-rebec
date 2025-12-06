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
	flagTaskNextFromID string
	flagTaskNextLevel  string
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Find the next task replacing a given task (by level or latest)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTaskNextFromID) == "" {
			return errors.New("--id is required")
		}
		lvl := strings.ToLower(strings.TrimSpace(flagTaskNextLevel))
		if lvl == "" {
			lvl = "latest"
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
		var id string
		switch lvl {
		case "patch", "minor", "major":
			id, err = pgdao.FindNextByLevel(ctx, db, flagTaskNextFromID, lvl)
		case "latest":
			id, err = pgdao.FindLatestFrom(ctx, db, flagTaskNextFromID)
		default:
			return errors.New("--level must be one of: patch|minor|major|latest")
		}
		if err != nil {
			return fmt.Errorf("cmd=task next params={id:%s,level:%s}: %w", flagTaskNextFromID, lvl, err)
		}
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("cmd=task next no result params={id:%s,level:%s}", flagTaskNextFromID, lvl)
		}
		t, err := pgdao.GetTaskByID(ctx, db, id)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "next(%s) task id=%s variant=%q command=%q\n", lvl, t.ID, t.Variant, t.Command)
		out := map[string]any{"id": t.ID, "variant": t.Variant, "command": t.Command, "level": lvl}
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
	TaskCmd.AddCommand(nextCmd)
	nextCmd.Flags().StringVar(&flagTaskNextFromID, "id", "", "Current task UUID (required)")
	nextCmd.Flags().StringVar(&flagTaskNextLevel, "level", "latest", "Replacement level: patch|minor|major|latest (default latest)")
}
