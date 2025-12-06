package message

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
	flagMsgDelID            string
	flagMsgDelForce         bool
	flagMsgDelIgnoreMissing bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a message by id (asks for confirmation unless --force)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagMsgDelID) == "" {
			return errors.New("--id is required")
		}
		if !flagMsgDelForce {
			fmt.Fprintf(os.Stderr, "About to delete message id=%d.\n", flagMsgDelID)
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
		affected, err := pgdao.DeleteMessage(ctx, db, flagMsgDelID)
		if err != nil {
			return err
		}
		if affected == 0 {
			if flagMsgDelIgnoreMissing {
				fmt.Fprintf(os.Stderr, "message id=%d not found; ignoring\n", flagMsgDelID)
				out := map[string]any{"status": "not_found_ignored", "id": flagMsgDelID}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return fmt.Errorf("message id=%d not found", flagMsgDelID)
		}
		fmt.Fprintf(os.Stderr, "message deleted id=%d\n", flagMsgDelID)
		out := map[string]any{"status": "deleted", "deleted": true, "id": flagMsgDelID}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	MessageCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&flagMsgDelID, "id", "", "Message UUID (required)")
	deleteCmd.Flags().BoolVar(&flagMsgDelForce, "force", false, "Do not prompt for confirmation")
	deleteCmd.Flags().BoolVar(&flagMsgDelIgnoreMissing, "ignore-missing", false, "Do not error if message does not exist")
}
