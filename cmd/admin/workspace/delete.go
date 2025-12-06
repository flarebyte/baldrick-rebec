package workspace

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
	flagWSDelID            string
	flagWSDelForce         bool
	flagWSDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a workspace by id (asks for confirmation unless --force)",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := strings.TrimSpace(flagWSDelID)
		if id == "" {
			return errors.New("--id is required")
		}
		if !flagWSDelForce {
			fmt.Fprintf(os.Stderr, "About to delete workspace %q.\n", id)
			fmt.Fprint(os.Stderr, "Type the workspace id to confirm: ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			if strings.TrimSpace(line) != id {
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
		affected, err := pgdao.DeleteWorkspace(ctx, db, id)
		if err != nil {
			return err
		}
		if affected == 0 {
			if flagWSDelIgnoreMissing {
				fmt.Fprintf(os.Stderr, "workspace %q not found; ignoring\n", id)
				out := map[string]any{"status": "not_found_ignored", "id": id}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return fmt.Errorf("workspace %q not found", id)
		}
		// Human-readable
		fmt.Fprintf(os.Stderr, "workspace deleted id=%q\n", id)
		// JSON
		out := map[string]any{"status": "deleted", "id": id, "deleted": true}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	WorkspaceCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&flagWSDelID, "id", "", "Workspace UUID (required)")
	deleteCmd.Flags().BoolVar(&flagWSDelForce, "force", false, "Do not prompt for confirmation")
	deleteCmd.Flags().BoolVar(&flagWSDelIgnoreMissing, "ignore-missing", false, "Do not error if workspace does not exist")
}
