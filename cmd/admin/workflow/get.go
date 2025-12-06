package workflow

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
	flagWFGetName string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a workflow by name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagWFGetName) == "" {
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
		w, err := pgdao.GetWorkflowByName(ctx, db, flagWFGetName)
		if err != nil {
			return err
		}
		// Human-friendly line on stderr
		var desc, notes string
		if w.Description.Valid {
			desc = w.Description.String
		}
		if w.Notes.Valid {
			notes = w.Notes.String
		}
		fmt.Fprintf(os.Stderr, "workflow name=%q title=%q\n", w.Name, w.Title)
		// JSON on stdout
		out := map[string]any{
			"name":  w.Name,
			"title": w.Title,
		}
		if w.Created.Valid {
			out["created"] = w.Created.Time.Format(time.RFC3339Nano)
		}
		if w.Updated.Valid {
			out["updated"] = w.Updated.Time.Format(time.RFC3339Nano)
		}
		if desc != "" {
			out["description"] = desc
		}
		if notes != "" {
			out["notes"] = notes
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	WorkflowCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&flagWFGetName, "name", "", "Workflow unique name (required)")
}
