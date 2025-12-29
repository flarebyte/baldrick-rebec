package tool

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
	flagToolGetName string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a tool by name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagToolGetName) == "" {
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
		t, err := pgdao.GetToolByName(ctx, db, flagToolGetName)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "tool name=%q role=%q\n", t.Name, t.RoleName)
		out := map[string]any{"name": t.Name, "title": t.Title, "role": t.RoleName}
		if t.Description.Valid {
			out["description"] = t.Description.String
		}
		if t.Notes.Valid {
			out["notes"] = t.Notes.String
		}
		if len(t.Tags) > 0 {
			out["tags"] = t.Tags
		}
		if len(t.Settings) > 0 {
			out["settings"] = t.Settings
		}
		if t.ToolType.Valid {
			out["type"] = t.ToolType.String
		}
		if t.Created.Valid {
			out["created"] = t.Created.Time.Format(time.RFC3339Nano)
		}
		if t.Updated.Valid {
			out["updated"] = t.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	ToolCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&flagToolGetName, "name", "", "Tool unique name (required)")
}
