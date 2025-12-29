package project

import (
	"bufio"
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
	flagPrjDelName          string
	flagPrjDelRole          string
	flagPrjDelForce         bool
	flagPrjDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a project by name and role (asks for confirmation unless --force)",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(flagPrjDelName)
		role := strings.TrimSpace(flagPrjDelRole)
		if name == "" {
			return errors.New("--name is required")
		}
		if role == "" {
			return errors.New("--role is required")
		}
		if !flagPrjDelForce {
			fmt.Fprintf(os.Stderr, "About to delete project %q for role %q.\n", name, role)
			fmt.Fprint(os.Stderr, "Type the project name to confirm: ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			if strings.TrimSpace(line) != name {
				return errors.New("confirmation did not match; aborting")
			}
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
		affected, err := pgdao.DeleteProject(ctx, db, name, role)
		if err != nil {
			return err
		}
		if affected == 0 {
			if flagPrjDelIgnoreMissing {
				fmt.Fprintf(os.Stderr, "project %q (role=%q) not found; ignoring\n", name, role)
				out := map[string]any{"status": "not_found_ignored", "name": name, "role": role}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return fmt.Errorf("project %q (role=%q) not found", name, role)
		}
		// Human-readable
		fmt.Fprintf(os.Stderr, "project deleted name=%q role=%q\n", name, role)
		// JSON
		out := map[string]any{"status": "deleted", "name": name, "role": role, "deleted": true}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	ProjectCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&flagPrjDelName, "name", "", "Project unique name within role (required)")
	deleteCmd.Flags().StringVar(&flagPrjDelRole, "role", "", "Role name (required)")
	deleteCmd.Flags().BoolVar(&flagPrjDelForce, "force", false, "Do not prompt for confirmation")
	deleteCmd.Flags().BoolVar(&flagPrjDelIgnoreMissing, "ignore-missing", false, "Do not error if project does not exist")
}
