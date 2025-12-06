package queue

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
	flagQTakeID string
)

var takeCmd = &cobra.Command{
	Use:   "take",
	Short: "Take (start) a queue item by id (status->Running)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagQTakeID) == "" {
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
		q, err := pgdao.TakeQueue(ctx, db, flagQTakeID)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "queue taken id=%s status=%s\n", q.ID, q.Status)
		out := map[string]any{"id": q.ID, "status": q.Status}
		if q.InQueueSince.Valid {
			out["inQueueSince"] = q.InQueueSince.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	QueueCmd.AddCommand(takeCmd)
	takeCmd.Flags().StringVar(&flagQTakeID, "id", "", "Queue UUID (required)")
}
