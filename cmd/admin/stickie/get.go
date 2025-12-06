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
	flagStGetID string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a stickie by id",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagStGetID) == "" {
			return errors.New("--id is required")
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
		s, err := pgdao.GetStickieByID(ctx, db, flagStGetID)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "stickie id=%s blackboard=%s\n", s.ID, s.BlackboardID)
		out := map[string]any{"id": s.ID, "blackboard_id": s.BlackboardID, "edit_count": s.EditCount}
		if s.TopicName.Valid {
			out["topic_name"] = s.TopicName.String
		}
		if s.TopicRoleName.Valid {
			out["topic_role_name"] = s.TopicRoleName.String
		}
		if s.Note.Valid {
			out["note"] = s.Note.String
		}
		if len(s.Labels) > 0 {
			out["labels"] = s.Labels
		}
		if s.CreatedByTaskID.Valid {
			out["created_by_task_id"] = s.CreatedByTaskID.String
		}
		if s.PriorityLevel.Valid {
			out["priority_level"] = s.PriorityLevel.String
		}
		if s.Score.Valid {
			out["score"] = s.Score.Float64
		}
		if s.Created.Valid {
			out["created"] = s.Created.Time.Format(time.RFC3339Nano)
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
	StickieCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&flagStGetID, "id", "", "Stickie UUID (required)")
}
