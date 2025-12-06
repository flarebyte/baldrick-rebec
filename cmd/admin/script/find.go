package script

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
	flagFindName     string
	flagFindVariant  string
	flagFindArchived bool
	flagFindRole     string
)

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find a single script by complex name (name + variant)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagFindName) == "" {
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

		var s *pgdao.Script
		if strings.TrimSpace(flagFindRole) != "" {
			s, err = pgdao.GetScriptByComplexNameRole(ctx, db, flagFindName, flagFindVariant, flagFindArchived, flagFindRole)
		} else {
			s, err = pgdao.GetScriptByComplexName(ctx, db, flagFindName, flagFindVariant, flagFindArchived)
		}
		if err != nil {
			return err
		}
		// stderr summary
		fmt.Fprintf(os.Stderr, "script id=%s name=%q variant=%q archived=%t\n", s.ID, s.ComplexName.Name, s.ComplexName.Variant, s.Archived)
		// stdout JSON
		out := map[string]any{
			"id":         s.ID,
			"name":       s.ComplexName.Name,
			"variant":    s.ComplexName.Variant,
			"archived":   s.Archived,
			"title":      s.Title,
			"role":       s.RoleName,
			"content_id": s.ScriptContentID,
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
	ScriptCmd.AddCommand(findCmd)
	findCmd.Flags().StringVar(&flagFindName, "name", "", "Complex name: name (required)")
	findCmd.Flags().StringVar(&flagFindVariant, "variant", "", "Complex name: variant (optional; default empty)")
	findCmd.Flags().BoolVar(&flagFindArchived, "archived", false, "Search archived scripts instead of active ones")
	findCmd.Flags().StringVar(&flagFindRole, "role", "", "Optional role_name scope filter")
}
