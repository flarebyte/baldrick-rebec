package pkg

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
	flagPkgGetID      string
	flagPkgGetRole    string
	flagPkgGetVariant string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a package by id or by (role, variant)",
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
		var p *pgdao.Package
		if strings.TrimSpace(flagPkgGetID) != "" {
			p, err = pgdao.GetPackageByID(ctx, db, flagPkgGetID)
		} else {
			if strings.TrimSpace(flagPkgGetRole) == "" || strings.TrimSpace(flagPkgGetVariant) == "" {
				return errors.New("provide --id or both --role and --variant")
			}
			p, err = pgdao.GetPackageByKey(ctx, db, flagPkgGetRole, flagPkgGetVariant)
		}
		if err != nil {
			return err
		}
		// Human
		fmt.Fprintf(os.Stderr, "package id=%s role_name=%q task_id=%s\n", p.ID, p.RoleName, p.TaskID)
		// JSON
		out := map[string]any{"id": p.ID, "role_name": p.RoleName, "task_id": p.TaskID}
		if p.Created.Valid {
			out["created"] = p.Created.Time.Format(time.RFC3339Nano)
		}
		if p.Updated.Valid {
			out["updated"] = p.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	PackageCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&flagPkgGetID, "id", "", "Package UUID")
	getCmd.Flags().StringVar(&flagPkgGetRole, "role", "", "Role name (with --variant)")
	getCmd.Flags().StringVar(&flagPkgGetVariant, "variant", "", "Variant (with --role)")
}
