package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	flagQPeekLimit  int
	flagQPeekStatus string
	flagQPeekOutput string
)

var peekCmd = &cobra.Command{
	Use:   "peek",
	Short: "Peek at the oldest queue items",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		items, err := pgdao.PeekQueues(ctx, db, flagQPeekLimit, flagQPeekStatus)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "queues: %d\n", len(items))
		if strings.ToLower(strings.TrimSpace(flagQPeekOutput)) == "json" {
			arr := make([]map[string]any, 0, len(items))
			for _, q := range items {
				m := map[string]any{"id": q.ID, "status": q.Status}
				if q.InQueueSince.Valid {
					m["inQueueSince"] = q.InQueueSince.Time.Format(time.RFC3339Nano)
				}
				if q.Description.Valid {
					m["description"] = q.Description.String
				}
				arr = append(arr, m)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "STATUS", "SINCE"})
		for _, q := range items {
			since := ""
			if q.InQueueSince.Valid {
				since = q.InQueueSince.Time.Format(time.RFC3339)
			}
			table.Append([]string{q.ID, q.Status, since})
		}
		table.Render()
		return nil
	},
}

func init() {
	QueueCmd.AddCommand(peekCmd)
	peekCmd.Flags().IntVar(&flagQPeekLimit, "limit", 10, "Max items to peek")
	peekCmd.Flags().StringVar(&flagQPeekStatus, "status", "", "Filter by status")
	peekCmd.Flags().StringVar(&flagQPeekOutput, "output", "table", "Output: table|json")
}
