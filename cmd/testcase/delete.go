package testcase

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
	flagTCDeleteID     string
	flagTCDeleteForce  bool
	flagTCDeleteIgnore bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a testcase by id",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := strings.TrimSpace(flagTCDeleteID)
		if id == "" {
			return errors.New("--id is required")
		}
		if !flagTCDeleteForce {
			fmt.Fprintf(os.Stderr, "About to delete testcase %q. Type yes to confirm: ", id)
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(line)) != "yes" {
				return errors.New("confirmation not 'yes'")
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
		n, err := pgdao.DeleteTestcase(ctx, db, id)
		if err != nil {
			return err
		}
		if n == 0 {
			if flagTCDeleteIgnore {
				fmt.Fprintf(os.Stderr, "testcase %q not found; ignoring\n", id)
				out := map[string]any{"status": "not_found_ignored"}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return fmt.Errorf("testcase %q not found", id)
		}
		fmt.Fprintf(os.Stderr, "testcase deleted id=%s\n", id)
		out := map[string]any{"status": "deleted", "id": id, "deleted": true}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	TestcaseCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&flagTCDeleteID, "id", "", "Testcase UUID (required)")
	deleteCmd.Flags().BoolVar(&flagTCDeleteForce, "force", false, "Do not prompt for confirmation")
	deleteCmd.Flags().BoolVar(&flagTCDeleteIgnore, "ignore-missing", false, "Do not error if not found")
}
