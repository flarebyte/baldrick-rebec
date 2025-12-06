package topic

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
	flagTopicGetName string
	flagTopicGetRole string
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a topic by name and role",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagTopicGetName) == "" {
			return errors.New("--name is required")
		}
		if strings.TrimSpace(flagTopicGetRole) == "" {
			return errors.New("--role is required")
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
		t, err := pgdao.GetTopicByKey(ctx, db, flagTopicGetName, flagTopicGetRole)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "topic name=%q role=%q\n", t.Name, t.RoleName)
		out := map[string]any{"name": t.Name, "role": t.RoleName, "title": t.Title}
		if t.Description.Valid && t.Description.String != "" {
			out["description"] = t.Description.String
		}
		if t.Notes.Valid && t.Notes.String != "" {
			out["notes"] = t.Notes.String
		}
		if t.Created.Valid {
			out["created"] = t.Created.Time.Format(time.RFC3339Nano)
		}
		if t.Updated.Valid {
			out["updated"] = t.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	TopicCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&flagTopicGetName, "name", "", "Topic unique name within role (required)")
	getCmd.Flags().StringVar(&flagTopicGetRole, "role", "", "Role name (required)")
}
