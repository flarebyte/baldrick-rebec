package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagBackupOutput string
)

// deterministic table order for backup/restore
var backupTables = []string{
    "roles",
    "workflows",
    "tags",
    "projects",
    "tools",
    "blackboards",
    "stickies",
    "conversations",
	"experiments",
	"task_variants",
	"tasks",
	"scripts_content",
	"scripts",
	"messages_content",
	"messages",
	"workspaces",
	"packages",
	"testcases",
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup key tables to JSON (stdout or file)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		data := map[string][]json.RawMessage{}
		for _, tbl := range backupTables {
			rows, err := db.Query(ctx, "SELECT to_jsonb(t) FROM "+tbl+" t")
			if err != nil {
				return fmt.Errorf("query %s: %w", tbl, err)
			}
			defer rows.Close()
			col := make([]json.RawMessage, 0)
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					return err
				}
				col = append(col, json.RawMessage(raw))
			}
			if err := rows.Err(); err != nil {
				return err
			}
			data[tbl] = col
		}
		var out *os.File = os.Stdout
		if strings.TrimSpace(flagBackupOutput) != "" {
			f, err := os.Create(flagBackupOutput)
			if err != nil {
				return err
			}
			defer f.Close()
			out = f
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	},
}

func init() {
	DBCmd.AddCommand(backupCmd)
	backupCmd.Flags().StringVar(&flagBackupOutput, "output", "", "Output file path (default stdout)")
}
