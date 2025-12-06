package experiment

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
	flagExpDelID            string
	flagExpDelForce         bool
	flagExpDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an experiment by id (asks for confirmation unless --force)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagExpDelID) == "" {
			return errors.New("--id is required")
		}
		if !flagExpDelForce {
			fmt.Fprintf(os.Stderr, "About to delete experiment id=%d.\n", flagExpDelID)
			fmt.Fprint(os.Stderr, "Type 'yes' to confirm: ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(line)) != "yes" {
				return errors.New("confirmation not 'yes'; aborting")
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
		affected, err := pgdao.DeleteExperiment(ctx, db, flagExpDelID)
		if err != nil {
			return err
		}
		if affected == 0 {
			if flagExpDelIgnoreMissing {
				fmt.Fprintf(os.Stderr, "experiment id=%d not found; ignoring\n", flagExpDelID)
				out := map[string]any{"status": "not_found_ignored", "id": flagExpDelID}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return fmt.Errorf("experiment id=%d not found", flagExpDelID)
		}
		fmt.Fprintf(os.Stderr, "experiment deleted id=%d\n", flagExpDelID)
		out := map[string]any{"status": "deleted", "deleted": true, "id": flagExpDelID}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	ExperimentCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&flagExpDelID, "id", "", "Experiment UUID (required)")
	deleteCmd.Flags().BoolVar(&flagExpDelForce, "force", false, "Do not prompt for confirmation")
	deleteCmd.Flags().BoolVar(&flagExpDelIgnoreMissing, "ignore-missing", false, "Do not error if experiment does not exist")
}
