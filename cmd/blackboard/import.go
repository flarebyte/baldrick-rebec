package blackboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

var (
	flagImportDetailed bool
)

// importCmd implements: rbc blackboard import <folder>
// Creates a new blackboard and stickies using the IDs provided in YAML files.
// Preconditions:
// - folder/blackboard.yaml must exist and contain an id
// - No blackboard with that id exists in DB
// - Any stickie YAML with id: must not exist in DB
var importCmd = &cobra.Command{
	Use:   "import <folder>",
	Short: "Import a blackboard and stickies from a folder (IDs preserved)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		folder := strings.TrimSpace(args[0])
		if folder == "" {
			return errors.New("folder is required")
		}
		if filepath.IsAbs(folder) || strings.HasPrefix(filepath.Clean(folder), "..") {
			return errors.New("folder must be a relative path inside the workspace")
		}

		// Load YAMLs from folder
		bbY, err := readLocalBlackboardYAML(filepath.Join(folder, "blackboard.yaml"))
		if err != nil {
			return fmt.Errorf("read blackboard.yaml: %w", err)
		}
		if strings.TrimSpace(bbY.ID) == "" {
			return errors.New("blackboard.yaml missing required id")
		}

		// Open DB
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		db, err := pgdao.OpenApp(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		// Run a diff preview first to show intent
		_ = runBlackboardDiff(bbY.ID, folder, flagImportDetailed, true)

		// Existence checks: blackboard and stickies must NOT exist
		if _, err := pgdao.GetBlackboardByID(ctx, db, bbY.ID); err == nil {
			return fmt.Errorf("blackboard already exists: id=%s", bbY.ID)
		}

		// Load local stickies
		byID, anon, err := loadLocalStickies(folder, true)
		if err != nil {
			return err
		}
		if len(anon) > 0 {
			return fmt.Errorf("found %d stickie yaml without id; import requires explicit ids", len(anon))
		}
		// Check none of the stickie ids exist
		for id := range byID {
			if _, err := pgdao.GetStickieByID(ctx, db, id); err == nil {
				return fmt.Errorf("stickie already exists: id=%s", id)
			}
		}

		// Insert blackboard with provided ID
		if err := insertBlackboardWithID(ctx, db, bbY); err != nil {
			return fmt.Errorf("insert blackboard: %w", err)
		}
		fmt.Fprintf(os.Stderr, "imported blackboard id=%s role=%q\n", bbY.ID, bbY.Role)

		// Insert stickies with provided IDs, stable order
		ids := make([]string, 0, len(byID))
		for id := range byID {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			rec := byID[id]
			if strings.TrimSpace(rec.yaml.ID) == "" {
				return fmt.Errorf("stickie %s missing id unexpectedly", rec.filename)
			}
			if err := insertStickieWithID(ctx, db, bbY.ID, rec.yaml); err != nil {
				return fmt.Errorf("insert stickie id=%s from %s: %w", rec.yaml.ID, rec.filename, err)
			}
			fmt.Fprintf(os.Stderr, "imported stickie id=%s file=%s\n", rec.yaml.ID, rec.filename)
		}

		return nil
	},
}

func init() {
	BlackboardCmd.AddCommand(importCmd)
	importCmd.Flags().BoolVar(&flagImportDetailed, "detailed", false, "Show detailed diff before importing")
}

// insertBlackboardWithID inserts a blackboard row using the ID from YAML.
func insertBlackboardWithID(ctx context.Context, db *pgxpool.Pool, y blackboardYAML) error {
	// Basic validation
	if strings.TrimSpace(y.ID) == "" || strings.TrimSpace(y.Role) == "" {
		return errors.New("blackboard yaml requires id and role")
	}
	// Prepare fields
	conv := strings.TrimSpace(deref(y.Conversation))
	proj := strings.TrimSpace(deref(y.Project))
	task := strings.TrimSpace(deref(y.TaskID))
	bg := strings.TrimSpace(derefLS(y.Background))
	gl := strings.TrimSpace(derefLS(y.Guidelines))
	lc := strings.TrimSpace(deref(y.Lifecycle))
	q := `INSERT INTO blackboards (id, role_name, conversation_id, project_name, task_id, background, guidelines, lifecycle)
          VALUES ($1::uuid, $2, CASE WHEN $3='' THEN NULL ELSE $3::uuid END, NULLIF($4,''), CASE WHEN $5='' THEN NULL ELSE $5::uuid END, NULLIF($6,''), NULLIF($7,''), NULLIF($8,''))`
	_, err := db.Exec(ctx, q, y.ID, y.Role, conv, proj, task, bg, gl, lc)
	return err
}

// insertStickieWithID inserts a stickie row using the ID from YAML under the given blackboard UUID.
func insertStickieWithID(ctx context.Context, db *pgxpool.Pool, blackboardID string, y stickieYAML) error {
	if strings.TrimSpace(y.ID) == "" {
		return errors.New("stickie yaml requires id")
	}
	// Prepare fields
	note := strings.TrimSpace(getLitPtr(y.Note))
	code := strings.TrimSpace(getStrPtr(y.Code))
	labels := y.Labels
	sort.Strings(labels)
	ctask := strings.TrimSpace(getStrPtr(y.CreatedByTask))
	prio := strings.TrimSpace(getStrPtr(y.Priority))
	var score *float64 = y.Score
	name := strings.TrimSpace(getStrPtr(y.Name))
	archived := y.Archived
	q := `INSERT INTO stickies (id, blackboard_id, note, code, labels, created_by_task_id, priority_level, score, name, archived)
          VALUES ($1::uuid, $2::uuid, NULLIF($3,''), NULLIF($4,''), COALESCE($5, ARRAY[]::text[]), CASE WHEN $6='' THEN NULL ELSE $6::uuid END, NULLIF($7,''), $8::double precision, NULLIF($9,''), COALESCE($10,false))`
	var lblParam any
	if len(labels) > 0 {
		lblParam = labels
	} else {
		lblParam = nil
	}
	_, err := db.Exec(ctx, q, y.ID, blackboardID, note, code, lblParam, ctask, prio, score, name, archived)
	return err
}
