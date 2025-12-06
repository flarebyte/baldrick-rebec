package db

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
	flagSearchText string
	flagSearchTopK int
	flagSearchJSON bool
)

type searchResult struct {
	ID      int64  `json:"id"`
	Preview string `json:"preview"`
}

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search messages content (PG-only text search)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagSearchText) == "" {
			return errors.New("--text is required")
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
		// Simple FTS using plainto_tsquery on 'simple' config
		q := `SELECT id, substr(text_content,1,160) AS preview
              FROM messages_content
              WHERE to_tsvector('simple', text_content) @@ plainto_tsquery('simple', $1)
              LIMIT $2`
		rows, err := db.Query(ctx, q, flagSearchText, flagSearchTopK)
		if err != nil {
			return err
		}
		defer rows.Close()
		results := []searchResult{}
		for rows.Next() {
			var r searchResult
			if err := rows.Scan(&r.ID, &r.Preview); err != nil {
				return err
			}
			results = append(results, r)
		}
		if flagSearchJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}
		for _, r := range results {
			fmt.Fprintf(os.Stdout, "%s\t%s\n", r.ID, r.Preview)
		}
		return nil
	},
}

func init() {
	DBCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVar(&flagSearchText, "text", "", "Text query for FTS")
	searchCmd.Flags().IntVar(&flagSearchTopK, "topk", 10, "Max number of results")
	searchCmd.Flags().BoolVar(&flagSearchJSON, "json", false, "Output results as JSON")
}
