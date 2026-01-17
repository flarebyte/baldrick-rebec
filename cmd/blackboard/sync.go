package blackboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

var (
	flagSyncDelete bool
	flagSyncDryRun bool
)

// syncCmd implements: rbc blackboard sync id:UUID folder:relative/path
// For now only supports id -> folder direction.
var syncCmd = &cobra.Command{
	Use:   "sync <source> <target>",
	Short: "Sync a blackboard and stickies between id and folder",
	Long:  "Sync a blackboard and its stickies between an id:UUID and a folder:RELATIVE_PATH. Supports id->folder and folder->id.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := parseEndpoint(args[0])
		if err != nil {
			return err
		}
		dst, err := parseEndpoint(args[1])
		if err != nil {
			return err
		}
		// Disallow same-kind syncs explicitly for clarity
		if src.kind == dst.kind {
			return fmt.Errorf("cannot sync %s -> %s; use id:UUID->folder:PATH or folder:PATH->id:UUID", kindString(src.kind), kindString(dst.kind))
		}

		if src.kind == epID && dst.kind == epFolder {
			return syncIDToFolder(src.value, dst.value, flagSyncDelete, flagSyncDryRun)
		}
		if src.kind == epFolder && dst.kind == epID {
			return syncFolderToID(src.value, dst.value, flagSyncDryRun)
		}

		return errors.New("supported directions: id:UUID->folder:PATH and folder:PATH->id:UUID")
	},
}

func init() {
	BlackboardCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&flagSyncDelete, "delete", false, "Delete files at destination that are not present at source")
	syncCmd.Flags().BoolVar(&flagSyncDryRun, "dry-run", false, "Show what would change without writing or deleting")
}

type endpointKind int

const (
	epUnknown endpointKind = iota
	epID
	epFolder
)

type endpoint struct {
	kind  endpointKind
	value string
}

func kindString(k endpointKind) string {
	switch k {
	case epID:
		return "id"
	case epFolder:
		return "folder"
	default:
		return "unknown"
	}
}

func parseEndpoint(s string) (endpoint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return endpoint{}, errors.New("empty endpoint")
	}
	if strings.HasPrefix(s, "id:") {
		v := strings.TrimSpace(strings.TrimPrefix(s, "id:"))
		if v == "" {
			return endpoint{}, errors.New("id endpoint missing UUID")
		}
		return endpoint{kind: epID, value: v}, nil
	}
	if strings.HasPrefix(s, "folder:") {
		v := strings.TrimSpace(strings.TrimPrefix(s, "folder:"))
		if v == "" {
			return endpoint{}, errors.New("folder endpoint missing path")
		}
		if filepath.IsAbs(v) {
			return endpoint{}, errors.New("folder path must be relative")
		}
		return endpoint{kind: epFolder, value: v}, nil
	}
	return endpoint{}, fmt.Errorf("unsupported endpoint %q (use id:<uuid> or folder:<path>)", s)
}

// Local YAML structures
type blackboardYAML struct {
	ID           string  `yaml:"id"`
	StoreID      string  `yaml:"store_id"`
	Role         string  `yaml:"role"`
	Conversation *string `yaml:"conversation_id,omitempty"`
	Project      *string `yaml:"project,omitempty"`
	TaskID       *string `yaml:"task_id,omitempty"`
	Background   *string `yaml:"background,omitempty"`
	Guidelines   *string `yaml:"guidelines,omitempty"`
	Created      *string `yaml:"created,omitempty"`
	Updated      *string `yaml:"updated,omitempty"`
}

type stickieComplexNameYAML struct {
	Name    string `yaml:"name"`
	Variant string `yaml:"variant"`
}

type stickieYAML struct {
	ID            string                 `yaml:"id"`
	TopicName     *string                `yaml:"topic_name,omitempty"`
	TopicRole     *string                `yaml:"topic_role_name,omitempty"`
	Note          *string                `yaml:"note,omitempty"`
	Code          *string                `yaml:"code,omitempty"`
	Labels        []string               `yaml:"labels,omitempty"`
	Created       *string                `yaml:"created,omitempty"`
	Updated       *string                `yaml:"updated,omitempty"`
	CreatedByTask *string                `yaml:"created_by_task_id,omitempty"`
	EditCount     int                    `yaml:"edit_count,omitempty"`
	Priority      *string                `yaml:"priority_level,omitempty"`
	Score         *float64               `yaml:"score,omitempty"`
	ComplexName   stickieComplexNameYAML `yaml:"complex_name"`
	Archived      bool                   `yaml:"archived"`
}

type minimalUpdatedYAML struct {
	Updated *string `yaml:"updated"`
}

func syncIDToFolder(blackboardID, relFolder string, allowDelete, dryRun bool) error {
	// Prepare destination directory
	destDir := filepath.Clean(relFolder)
	if strings.HasPrefix(destDir, "..") {
		return errors.New("folder path must not escape current directory")
	}
	if !dryRun {
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return fmt.Errorf("create dest folder: %w", err)
		}
	}

	// Open DB
	cfg, err := cfgpkg.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := pgdao.OpenApp(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	// Get blackboard
	b, err := pgdao.GetBlackboardByID(ctx, db, blackboardID)
	if err != nil {
		return err
	}

	// Prepare YAML for blackboard
	by := blackboardYAML{
		ID:      b.ID,
		StoreID: b.StoreID,
		Role:    b.RoleName,
	}
	if b.ConversationID.Valid && b.ConversationID.String != "" {
		v := b.ConversationID.String
		by.Conversation = &v
	}
	if b.ProjectName.Valid && b.ProjectName.String != "" {
		v := b.ProjectName.String
		by.Project = &v
	}
	if b.TaskID.Valid && b.TaskID.String != "" {
		v := b.TaskID.String
		by.TaskID = &v
	}
	if b.Background.Valid && b.Background.String != "" {
		v := b.Background.String
		by.Background = &v
	}
	if b.Guidelines.Valid && b.Guidelines.String != "" {
		v := b.Guidelines.String
		by.Guidelines = &v
	}
	if b.Created.Valid {
		v := b.Created.Time.Format(time.RFC3339Nano)
		by.Created = &v
	}
	if b.Updated.Valid {
		v := b.Updated.Time.Format(time.RFC3339Nano)
		by.Updated = &v
	}

	// Write blackboard.yaml if needed
	bbFile := filepath.Join(destDir, "blackboard.yaml")
	bbWrite := true
	if fi, err := os.Stat(bbFile); err == nil && fi.Mode().IsRegular() {
		// compare updated timestamps
		if newerOrEqual, err := destIsNewerOrEqual(bbFile, by.Updated); err == nil && newerOrEqual {
			bbWrite = false
		}
	}
	if bbWrite {
		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] write %s\n", bbFile)
		} else {
			if err := writeYAML(bbFile, by); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", bbFile)
		}
	} else {
		fmt.Fprintf(os.Stderr, "skip up-to-date %s\n", bbFile)
	}

	// Fetch stickies for the board (paged)
	stickies := make([]pgdao.Stickie, 0, 256)
	const page = 1000
	for off := 0; ; off += page {
		ss, err := pgdao.ListStickies(ctx, db, b.ID, "", "", page, off)
		if err != nil {
			return err
		}
		stickies = append(stickies, ss...)
		if len(ss) < page {
			break
		}
	}

	// Track seen stickie filenames to support --delete
	seen := make(map[string]struct{}, len(stickies))

	// Write each stickie YAML if newer
	for _, s := range stickies {
		sy := stickieYAML{
			ID:        s.ID,
			Labels:    s.Labels,
			EditCount: s.EditCount,
			ComplexName: stickieComplexNameYAML{
				Name:    s.ComplexName.Name,
				Variant: s.ComplexName.Variant,
			},
			Archived: s.Archived,
		}
		if s.TopicName.Valid && s.TopicName.String != "" {
			v := s.TopicName.String
			sy.TopicName = &v
		}
		if s.TopicRoleName.Valid && s.TopicRoleName.String != "" {
			v := s.TopicRoleName.String
			sy.TopicRole = &v
		}
		if s.Note.Valid && s.Note.String != "" {
			v := s.Note.String
			sy.Note = &v
		}
		if s.Code.Valid && s.Code.String != "" {
			v := s.Code.String
			sy.Code = &v
		}
		if s.Created.Valid {
			v := s.Created.Time.Format(time.RFC3339Nano)
			sy.Created = &v
		}
		if s.Updated.Valid {
			v := s.Updated.Time.Format(time.RFC3339Nano)
			sy.Updated = &v
		}
		if s.CreatedByTaskID.Valid && s.CreatedByTaskID.String != "" {
			v := s.CreatedByTaskID.String
			sy.CreatedByTask = &v
		}
		if s.PriorityLevel.Valid && s.PriorityLevel.String != "" {
			v := s.PriorityLevel.String
			sy.Priority = &v
		}
		if s.Score.Valid {
			v := s.Score.Float64
			sy.Score = &v
		}

		fn := filepath.Join(destDir, fmt.Sprintf("%s.stickie.yaml", s.ID))
		seen[filepath.Base(fn)] = struct{}{}
		write := true
		if fi, err := os.Stat(fn); err == nil && fi.Mode().IsRegular() {
			if newerOrEqual, err := destIsNewerOrEqual(fn, sy.Updated); err == nil && newerOrEqual {
				write = false
			}
		}
		if write {
			if dryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] write %s\n", fn)
			} else {
				if err := writeYAML(fn, sy); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "wrote %s\n", fn)
			}
		} else {
			fmt.Fprintf(os.Stderr, "skip up-to-date %s\n", fn)
		}
	}

	// Handle delete: any *.stickie.yaml in dest not in seen
	if allowDelete {
		entries, _ := os.ReadDir(destDir)
		// Collect and sort for stable output
		var toDelete []string
		for _, e := range entries {
			name := e.Name()
			if e.Type().IsRegular() && strings.HasSuffix(name, ".stickie.yaml") {
				if _, ok := seen[name]; !ok {
					toDelete = append(toDelete, filepath.Join(destDir, name))
				}
			}
		}
		sort.Strings(toDelete)
		for _, p := range toDelete {
			if dryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] delete %s\n", p)
			} else {
				if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				fmt.Fprintf(os.Stderr, "deleted %s\n", p)
			}
		}
	}

	return nil
}

func writeYAML(path string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	// Write atomically by temp + rename to reduce partial writes
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// destIsNewerOrEqual returns true if destination file's updated >= srcUpdated.
func destIsNewerOrEqual(path string, srcUpdated *string) (bool, error) {
	if srcUpdated == nil || *srcUpdated == "" {
		// No source timestamp; always write
		return false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var m minimalUpdatedYAML
	if err := yaml.Unmarshal(b, &m); err != nil {
		return false, nil // on error, prefer writing
	}
	if m.Updated == nil || *m.Updated == "" {
		return false, nil
	}
	// Parse timestamps
	srcT, err := time.Parse(time.RFC3339Nano, *srcUpdated)
	if err != nil {
		return false, nil
	}
	dstT, err := time.Parse(time.RFC3339Nano, *m.Updated)
	if err != nil {
		// Try RFC3339 without nanos as fallback
		if dstT2, e2 := time.Parse(time.RFC3339, *m.Updated); e2 == nil {
			dstT = dstT2
		} else {
			return false, nil
		}
	}
	return !dstT.Before(srcT), nil
}

// ---- folder -> id implementation ----

func syncFolderToID(relFolder, blackboardID string, dryRun bool) error {
	// Validate folder
	srcDir := filepath.Clean(relFolder)
	if strings.HasPrefix(srcDir, "..") {
		return errors.New("folder path must not escape current directory")
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

	// Ensure blackboard exists
	b, err := pgdao.GetBlackboardByID(ctx, db, blackboardID)
	if err != nil {
		return err
	}
	_ = b // currently unused beyond existence

	// Iterate stickie files
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read folder: %w", err)
	}
	// Stable order
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".stickie.yaml") {
			files = append(files, filepath.Join(srcDir, e.Name()))
		}
	}
	sort.Strings(files)

	for _, p := range files {
		y, err := readStickieYAML(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		// Always target destination blackboard; ignore/override yaml's blackboard_id
		// Security rule: if yaml has an id, it MUST already exist for this blackboard
		if strings.TrimSpace(y.ID) != "" {
			s, err := pgdao.GetStickieByID(ctx, db, y.ID)
			if err != nil {
				return fmt.Errorf("stickie %s referenced in %s does not exist: %w", y.ID, p, err)
			}
			if strings.TrimSpace(s.BlackboardID) != strings.TrimSpace(blackboardID) {
				return fmt.Errorf("security: stickie %s in %s does not belong to blackboard %s", y.ID, p, blackboardID)
			}
			// Decide update based on content hash
			hSrc := hashStickieYAML(y)
			hDst := hashStickieDB(*s)
			if hSrc == hDst {
				fmt.Fprintf(os.Stderr, "skip unchanged %s (id=%s)\n", p, y.ID)
				continue
			}
			if dryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] update stickie id=%s from %s\n", y.ID, p)
				continue
			}
			// Perform update
			upd := stickieFromYAMLForUpsert(y, blackboardID)
			upd.ID = y.ID
			if err := pgdao.UpsertStickie(ctx, db, &upd); err != nil {
				return fmt.Errorf("update stickie %s from %s: %w", y.ID, p, err)
			}
			fmt.Fprintf(os.Stderr, "updated stickie id=%s\n", y.ID)
		} else {
			// Create new stickie (no id assigned in yaml) -> assign UUID automatically
			if dryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] create stickie from %s\n", p)
				continue
			}
			ins := stickieFromYAMLForUpsert(y, blackboardID)
			ins.ID = "" // ensure create
			if err := pgdao.UpsertStickie(ctx, db, &ins); err != nil {
				return fmt.Errorf("create stickie from %s: %w", p, err)
			}
			fmt.Fprintf(os.Stderr, "created stickie id=%s from %s\n", ins.ID, p)
		}
	}

	return nil
}

func readStickieYAML(path string) (stickieYAML, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return stickieYAML{}, err
	}
	var y stickieYAML
	if err := yaml.Unmarshal(b, &y); err != nil {
		return stickieYAML{}, err
	}
	return y, nil
}

// stickieFromYAMLForUpsert maps YAML into pgdao.Stickie for UpsertStickie.
func stickieFromYAMLForUpsert(y stickieYAML, blackboardID string) pgdao.Stickie {
	var s pgdao.Stickie
	s.BlackboardID = blackboardID
	// optional simple values
	if y.TopicName != nil {
		s.TopicName.Valid = true
		s.TopicName.String = *y.TopicName
	}
	if y.TopicRole != nil {
		s.TopicRoleName.Valid = true
		s.TopicRoleName.String = *y.TopicRole
	}
	if y.Note != nil {
		s.Note.Valid = true
		s.Note.String = *y.Note
	}
	if y.Code != nil {
		s.Code.Valid = true
		s.Code.String = *y.Code
	}
	if len(y.Labels) > 0 {
		// ensure deterministic order; DB stores array w/o order guarantee
		cp := append([]string(nil), y.Labels...)
		sort.Strings(cp)
		s.Labels = cp
	}
	if y.CreatedByTask != nil {
		s.CreatedByTaskID.Valid = true
		s.CreatedByTaskID.String = *y.CreatedByTask
	}
	if y.Priority != nil {
		s.PriorityLevel.Valid = true
		s.PriorityLevel.String = *y.Priority
	}
	if y.Score != nil {
		s.Score.Valid = true
		s.Score.Float64 = *y.Score
	}
	s.ComplexName.Name = y.ComplexName.Name
	s.ComplexName.Variant = y.ComplexName.Variant
	s.Archived = y.Archived
	return s
}

// Hash utilities (SHA-256) for stickie content comparison
type stickieHashMaterial struct {
	TopicName     string   `json:"topic_name"`
	TopicRoleName string   `json:"topic_role_name"`
	Note          string   `json:"note"`
	Code          string   `json:"code"`
	Labels        []string `json:"labels"`
	CreatedByTask string   `json:"created_by_task_id"`
	PriorityLevel string   `json:"priority_level"`
	Score         *float64 `json:"score,omitempty"`
	ComplexName   struct {
		Name    string `json:"name"`
		Variant string `json:"variant"`
	} `json:"complex_name"`
	Archived bool `json:"archived"`
}

func hashStickieYAML(y stickieYAML) string {
	mat := stickieHashMaterial{}
	if y.TopicName != nil {
		mat.TopicName = *y.TopicName
	}
	if y.TopicRole != nil {
		mat.TopicRoleName = *y.TopicRole
	}
	if y.Note != nil {
		mat.Note = *y.Note
	}
	if y.Code != nil {
		mat.Code = *y.Code
	}
	if len(y.Labels) > 0 {
		cp := append([]string(nil), y.Labels...)
		sort.Strings(cp)
		mat.Labels = cp
	}
	if y.CreatedByTask != nil {
		mat.CreatedByTask = *y.CreatedByTask
	}
	if y.Priority != nil {
		mat.PriorityLevel = *y.Priority
	}
	if y.Score != nil {
		mat.Score = y.Score
	}
	mat.ComplexName.Name = y.ComplexName.Name
	mat.ComplexName.Variant = y.ComplexName.Variant
	mat.Archived = y.Archived
	return hashMaterial(mat)
}

func hashStickieDB(s pgdao.Stickie) string {
	mat := stickieHashMaterial{}
	if s.TopicName.Valid {
		mat.TopicName = s.TopicName.String
	}
	if s.TopicRoleName.Valid {
		mat.TopicRoleName = s.TopicRoleName.String
	}
	if s.Note.Valid {
		mat.Note = s.Note.String
	}
	if s.Code.Valid {
		mat.Code = s.Code.String
	}
	if len(s.Labels) > 0 {
		cp := append([]string(nil), s.Labels...)
		sort.Strings(cp)
		mat.Labels = cp
	}
	if s.CreatedByTaskID.Valid {
		mat.CreatedByTask = s.CreatedByTaskID.String
	}
	if s.PriorityLevel.Valid {
		mat.PriorityLevel = s.PriorityLevel.String
	}
	if s.Score.Valid {
		v := s.Score.Float64
		mat.Score = &v
	}
	mat.ComplexName.Name = s.ComplexName.Name
	mat.ComplexName.Variant = s.ComplexName.Variant
	mat.Archived = s.Archived
	return hashMaterial(mat)
}

func hashMaterial(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
