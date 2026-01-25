package stickie

import (
	"context"
	"database/sql"
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
	flagStID         string
	flagStBlackboard string
	// topic flags removed; use labels instead
	flagStNote      string
	flagStCode      string
	flagStLabels    []string
	flagStCreatedBy string
	flagStPriority  string
	flagStName      string
	flagStArchived  bool
	flagStScore     float64
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Create or update a stickie (by id)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagStID) == "" && strings.TrimSpace(flagStBlackboard) == "" {
			return errors.New("--blackboard is required when creating a stickie")
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

		st := &pgdao.Stickie{ID: strings.TrimSpace(flagStID)}
		if strings.TrimSpace(flagStBlackboard) != "" {
			st.BlackboardID = strings.TrimSpace(flagStBlackboard)
		}
		if strings.TrimSpace(flagStNote) != "" {
			st.Note = sql.NullString{String: flagStNote, Valid: true}
		}
		if strings.TrimSpace(flagStCode) != "" {
			st.Code = sql.NullString{String: flagStCode, Valid: true}
		}
		if len(flagStLabels) > 0 {
			st.Labels = flagStLabels
		}
		if strings.TrimSpace(flagStCreatedBy) != "" {
			st.CreatedByTaskID = sql.NullString{String: strings.TrimSpace(flagStCreatedBy), Valid: true}
		}
		if strings.TrimSpace(flagStPriority) != "" {
			st.PriorityLevel = sql.NullString{String: strings.ToLower(flagStPriority), Valid: true}
		}
		if strings.TrimSpace(flagStName) != "" {
			st.Name = sql.NullString{String: strings.TrimSpace(flagStName), Valid: true}
		}
		st.Archived = flagStArchived

		// Optional score; only set if flag provided
		if cmd.Flags().Changed("score") {
			st.Score = sql.NullFloat64{Float64: flagStScore, Valid: true}
		}

		if err := pgdao.UpsertStickie(ctx, db, st); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "stickie upserted id=%s blackboard=%s\n", st.ID, st.BlackboardID)
		out := map[string]any{"status": "upserted", "id": st.ID, "blackboard_id": st.BlackboardID, "edit_count": st.EditCount}
		if st.Created.Valid {
			out["created"] = st.Created.Time.Format(time.RFC3339Nano)
		}
		if st.Updated.Valid {
			out["updated"] = st.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	StickieCmd.AddCommand(setCmd)
	setCmd.Flags().StringVar(&flagStID, "id", "", "Stickie UUID (optional; when omitted, a new id is generated)")
	setCmd.Flags().StringVar(&flagStBlackboard, "blackboard", "", "Blackboard UUID (required on create)")
	// topics removed from stickies interface
	setCmd.Flags().StringVar(&flagStNote, "note", "", "Note text")
	setCmd.Flags().StringVar(&flagStCode, "code", "", "Code snippet (programming language)")
	setCmd.Flags().StringSliceVar(&flagStLabels, "labels", nil, "Labels (repeat or comma-separated)")
	setCmd.Flags().StringVar(&flagStCreatedBy, "created-by-task", "", "Creator task UUID (optional)")
	setCmd.Flags().StringVar(&flagStPriority, "priority", "", "Priority level: must, should, could, wont")
	setCmd.Flags().StringVar(&flagStName, "name", "", "Human-readable name (exact lookup key)")
	setCmd.Flags().BoolVar(&flagStArchived, "archived", false, "Mark stickie as archived (excluded from active lookups)")
	setCmd.Flags().Float64Var(&flagStScore, "score", 0, "Optimisation score (optional; double precision)")
}
