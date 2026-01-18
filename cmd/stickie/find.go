package stickie

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
	flagStFindName       string
	flagStFindArchived   bool
	flagStFindBlackboard string
)

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find a single stickie by name (optionally scoped to a blackboard)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagStFindName) == "" {
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

		var s *pgdao.Stickie
		if strings.TrimSpace(flagStFindBlackboard) != "" {
			s, err = pgdao.GetStickieByNameInBlackboard(ctx, db, flagStFindName, flagStFindArchived, flagStFindBlackboard)
		} else {
			s, err = pgdao.GetStickieByName(ctx, db, flagStFindName, flagStFindArchived)
		}
		if err != nil {
			return err
		}
		// stderr summary
		fmt.Fprintf(os.Stderr, "stickie id=%s board=%s name=%q archived=%t\n", s.ID, s.BlackboardID, s.Name.String, s.Archived)
		// stdout JSON
		out := map[string]any{
			"id":            s.ID,
			"blackboard_id": s.BlackboardID,
			"name":          s.Name.String,
			"archived":      s.Archived,
			"edit_count":    s.EditCount,
		}
		// topics removed
		if s.Score.Valid {
			out["score"] = s.Score.Float64
		}
		if s.Updated.Valid {
			out["updated"] = s.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	StickieCmd.AddCommand(findCmd)
	findCmd.Flags().StringVar(&flagStFindName, "name", "", "Stickie name (required)")
	findCmd.Flags().BoolVar(&flagStFindArchived, "archived", false, "Search archived stickies instead of active ones")
	findCmd.Flags().StringVar(&flagStFindBlackboard, "blackboard", "", "Optional blackboard UUID scope filter")
}
