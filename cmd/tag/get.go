package tag

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
	flagTagGetName string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a tag by name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTagGetName) == "" {
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
		t, err := pgdao.GetTagByName(ctx, db, flagTagGetName)
		if err != nil {
			return err
		}
		var desc, notes string
		if t.Description.Valid {
			desc = t.Description.String
		}
		if t.Notes.Valid {
			notes = t.Notes.String
		}
		// stderr line
		fmt.Fprintf(os.Stderr, "tag name=%q title=%q\n", t.Name, t.Title)
		// stdout JSON
		out := map[string]any{
			"name":  t.Name,
			"title": t.Title,
		}
		if t.Created.Valid {
			out["created"] = t.Created.Time.Format(time.RFC3339Nano)
		}
		if t.Updated.Valid {
			out["updated"] = t.Updated.Time.Format(time.RFC3339Nano)
		}
		if desc != "" {
			out["description"] = desc
		}
		if notes != "" {
			out["notes"] = notes
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	TagCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&flagTagGetName, "name", "", "Tag unique name (required)")
}
