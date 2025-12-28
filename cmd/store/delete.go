package store

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
	flagStoreDelName          string
	flagStoreDelRole          string
	flagStoreDelForce         bool
	flagStoreDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a store by name and role (asks for confirmation unless --force)",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(flagStoreDelName)
		role := strings.TrimSpace(flagStoreDelRole)
		if name == "" {
			return errors.New("--name is required")
		}
		if role == "" {
			return errors.New("--role is required")
		}
		if !flagStoreDelForce {
			fmt.Fprintf(os.Stderr, "About to delete store %q for role %q.\n", name, role)
			fmt.Fprint(os.Stderr, "Type the store name to confirm: ")
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
		affected, err := pgdao.DeleteStore(ctx, db, name, role)
		if err != nil {
			return err
		}
		if affected == 0 {
			if flagStoreDelIgnoreMissing {
				fmt.Fprintf(os.Stderr, "store %q (role=%q) not found; ignoring\n", name, role)
				out := map[string]any{"status": "not_found_ignored", "name": name, "role": role}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return fmt.Errorf("store %q (role=%q) not found", name, role)
		}
		// Human-readable
		fmt.Fprintf(os.Stderr, "store deleted name=%q role=%q\n", name, role)
		// JSON
		out := map[string]any{"status": "deleted", "name": name, "role": role, "deleted": true}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	StoreCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&flagStoreDelName, "name", "", "Store unique name within role (required)")
	deleteCmd.Flags().StringVar(&flagStoreDelRole, "role", "", "Role name (required)")
	deleteCmd.Flags().BoolVar(&flagStoreDelForce, "force", false, "Do not prompt for confirmation")
	deleteCmd.Flags().BoolVar(&flagStoreDelIgnoreMissing, "ignore-missing", false, "Do not error if store does not exist")
}
