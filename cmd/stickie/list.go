package stickie

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	flagStListLimit      int
	flagStListOffset     int
	flagStListOutput     string
	flagStListBlackboard string
	flagStListTopicName  string
	flagStListTopicRole  string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List stickies (optionally filter by blackboard or topic)",
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
		ss, err := pgdao.ListStickies(ctx, db, strings.TrimSpace(flagStListBlackboard), strings.TrimSpace(flagStListTopicName), strings.TrimSpace(flagStListTopicRole), flagStListLimit, flagStListOffset)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "stickies: %d\n", len(ss))
		out := strings.ToLower(strings.TrimSpace(flagStListOutput))
		if out == "json" {
			arr := make([]map[string]any, 0, len(ss))
			for _, s := range ss {
				item := map[string]any{"id": s.ID, "blackboard_id": s.BlackboardID, "edit_count": s.EditCount}
				if s.TopicName.Valid {
					item["topic_name"] = s.TopicName.String
				}
				if s.TopicRoleName.Valid {
					item["topic_role_name"] = s.TopicRoleName.String
				}
				if s.Updated.Valid {
					item["updated"] = s.Updated.Time.Format(time.RFC3339Nano)
				}
				if s.ComplexName.Name != "" || s.ComplexName.Variant != "" {
					item["name"] = s.ComplexName.Name
					item["variant"] = s.ComplexName.Variant
				}
				if s.Archived {
					item["archived"] = true
				}
				arr = append(arr, item)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}
		// table default
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "BLACKBOARD", "TOPIC", "NAME", "VARIANT", "UPDATED", "EDIT#"})
		for _, s := range ss {
			updated := ""
			if s.Updated.Valid {
				updated = s.Updated.Time.Format(time.RFC3339)
			}
			topic := ""
			if s.TopicName.Valid {
				topic = s.TopicName.String
			}
			table.Append([]string{s.ID, s.BlackboardID, topic, s.ComplexName.Name, s.ComplexName.Variant, updated, fmt.Sprintf("%d", s.EditCount)})
		}
		table.Render()
		return nil
	},
}

func init() {
	StickieCmd.AddCommand(listCmd)
	listCmd.Flags().IntVar(&flagStListLimit, "limit", 100, "Max number of rows")
	listCmd.Flags().IntVar(&flagStListOffset, "offset", 0, "Offset for pagination")
	listCmd.Flags().StringVar(&flagStListOutput, "output", "table", "Output format: table or json")
	listCmd.Flags().StringVar(&flagStListBlackboard, "blackboard", "", "Filter by blackboard UUID")
	listCmd.Flags().StringVar(&flagStListTopicName, "topic-name", "", "Filter by topic name")
	listCmd.Flags().StringVar(&flagStListTopicRole, "topic-role", "", "Filter by topic role name")
}
