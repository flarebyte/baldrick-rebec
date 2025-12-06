package role

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
	flagRoleGetName string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a role by name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagRoleGetName) == "" {
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
		r, err := pgdao.GetRoleByName(ctx, db, flagRoleGetName)
		if err != nil {
			return err
		}
		// Human
		fmt.Fprintf(os.Stderr, "role name=%q title=%q\n", r.Name, r.Title)
		// JSON
		out := map[string]any{"name": r.Name, "title": r.Title}
		if r.Description.Valid {
			out["description"] = r.Description.String
		}
		if r.Notes.Valid {
			out["notes"] = r.Notes.String
		}
		if len(r.Tags) > 0 {
			out["tags"] = r.Tags
		}
		if r.Created.Valid {
			out["created"] = r.Created.Time.Format(time.RFC3339Nano)
		}
		if r.Updated.Valid {
			out["updated"] = r.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	RoleCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&flagRoleGetName, "name", "", "Role name (required)")
}
