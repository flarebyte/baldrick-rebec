package blackboard

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

var (
	flagBBID           string
	flagBBRole         string
	flagBBConvID       string
	flagBBProject      string
	flagBBTaskID       string
	flagBBBackground   string
	flagBBGuidelines   string
	flagBBLifecycle    string
	flagBBCliInputYAML bool
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Create or update a blackboard (by id)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Optional: read blackboard.yaml from stdin when --cli-input-yaml is set
		var yml struct {
			ID           string  `yaml:"id"`
			Role         string  `yaml:"role"`
			Conversation *string `yaml:"conversation_id"`
			Project      *string `yaml:"project"`
			TaskID       *string `yaml:"task_id"`
			Background   *string `yaml:"background"`
			Guidelines   *string `yaml:"guidelines"`
			Lifecycle    *string `yaml:"lifecycle"`
		}
		if flagBBCliInputYAML {
			fi, err := os.Stdin.Stat()
			if err != nil {
				return err
			}
			if (fi.Mode() & os.ModeCharDevice) != 0 {
				return errors.New("--cli-input-yaml requires YAML on stdin (e.g., cat blackboard.yaml | rbc blackboard set --cli-input-yaml)")
			}
			var bld strings.Builder
			r := bufio.NewReader(os.Stdin)
			for {
				chunk, err := r.ReadString('\n')
				bld.WriteString(chunk)
				if err != nil {
					if err == io.EOF {
						break
					}
					return fmt.Errorf("read stdin: %w", err)
				}
			}
			if err := yaml.Unmarshal([]byte(bld.String()), &yml); err != nil {
				return fmt.Errorf("invalid blackboard yaml: %w", err)
			}
		}

		// Merge: explicit flags override YAML values
		role := strings.TrimSpace(flagBBRole)
		if role == "" && strings.TrimSpace(yml.Role) != "" {
			role = strings.TrimSpace(yml.Role)
		}
		id := strings.TrimSpace(flagBBID)
		if id == "" && strings.TrimSpace(yml.ID) != "" {
			id = strings.TrimSpace(yml.ID)
		}
		convID := strings.TrimSpace(flagBBConvID)
		if convID == "" && yml.Conversation != nil {
			convID = strings.TrimSpace(*yml.Conversation)
		}
		project := strings.TrimSpace(flagBBProject)
		if project == "" && yml.Project != nil {
			project = strings.TrimSpace(*yml.Project)
		}
		taskID := strings.TrimSpace(flagBBTaskID)
		if taskID == "" && yml.TaskID != nil {
			taskID = strings.TrimSpace(*yml.TaskID)
		}
		background := flagBBBackground
		if strings.TrimSpace(background) == "" && yml.Background != nil {
			background = *yml.Background
		}
		guidelines := flagBBGuidelines
		if strings.TrimSpace(guidelines) == "" && yml.Guidelines != nil {
			guidelines = *yml.Guidelines
		}
		lifecycle := flagBBLifecycle
		if strings.TrimSpace(lifecycle) == "" && yml.Lifecycle != nil {
			lifecycle = *yml.Lifecycle
		}

		if strings.TrimSpace(role) == "" {
			return errors.New("--role is required (provide flag or in YAML)")
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

		b := &pgdao.Blackboard{ID: id, RoleName: role}
		if convID != "" {
			b.ConversationID = sql.NullString{String: convID, Valid: true}
		}
		if project != "" {
			b.ProjectName = sql.NullString{String: project, Valid: true}
		}
		if taskID != "" {
			b.TaskID = sql.NullString{String: taskID, Valid: true}
		}
		if strings.TrimSpace(background) != "" {
			b.Background = sql.NullString{String: background, Valid: true}
		}
		if strings.TrimSpace(guidelines) != "" {
			b.Guidelines = sql.NullString{String: guidelines, Valid: true}
		}
		if strings.TrimSpace(lifecycle) != "" {
			b.Lifecycle = sql.NullString{String: lifecycle, Valid: true}
		}

		if err := pgdao.UpsertBlackboard(ctx, db, b); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "blackboard upserted id=%s role=%q\n", b.ID, b.RoleName)
		out := map[string]any{"status": "upserted", "id": b.ID, "role": b.RoleName}
		if b.Lifecycle.Valid {
			out["lifecycle"] = b.Lifecycle.String
		}
		if b.Created.Valid {
			out["created"] = b.Created.Time.Format(time.RFC3339Nano)
		}
		if b.Updated.Valid {
			out["updated"] = b.Updated.Time.Format(time.RFC3339Nano)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

func init() {
	BlackboardCmd.AddCommand(setCmd)
	setCmd.Flags().StringVar(&flagBBID, "id", "", "Blackboard UUID (optional; when omitted, a new id is generated)")
	setCmd.Flags().StringVar(&flagBBRole, "role", "", "Role name (required)")
	setCmd.Flags().StringVar(&flagBBConvID, "conversation", "", "Conversation UUID (optional)")
	setCmd.Flags().StringVar(&flagBBProject, "project", "", "Project name (optional; must exist for role)")
	setCmd.Flags().StringVar(&flagBBTaskID, "task", "", "Task UUID (optional)")
	setCmd.Flags().StringVar(&flagBBBackground, "background", "", "Background text")
	setCmd.Flags().StringVar(&flagBBGuidelines, "guidelines", "", "Guidelines text")
	setCmd.Flags().StringVar(&flagBBLifecycle, "lifecycle", "", "Lifecycle: permanent|yearly|quarterly|monthly|weekly|daily")
	setCmd.Flags().BoolVar(&flagBBCliInputYAML, "cli-input-yaml", false, "Read blackboard.yaml from stdin to set fields; flags override YAML")
}
