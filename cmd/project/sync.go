package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

var (
	flagPrjSyncDryRun bool
	flagPrjSyncRole   string
)

// project sync implements: rbc project sync name:<PROJECT_NAME> folder:<RELATIVE_PATH> [--dry-run]
// It exports a single project row to <SANITIZED_NAME>.project.yaml in the target folder.
var syncCmd = &cobra.Command{
	Use:   "sync <source> <target>",
	Short: "Sync a project from DB to folder",
	Long:  "Sync a project identified by name (and role) to a folder as <sanitized>.project.yaml.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, err := parsePrjEndpoint(args[0])
		if err != nil {
			return err
		}
		dst, err := parsePrjEndpoint(args[1])
		if err != nil {
			return err
		}
		if src.kind != epName || dst.kind != epFolder {
			return errors.New("supported direction: name:<PROJECT_NAME> -> folder:<RELATIVE_PATH>")
		}
		if strings.TrimSpace(flagPrjSyncRole) == "" {
			return errors.New("--role is required to select the project")
		}
		return syncNameToFolder(src.value, dst.value, flagPrjSyncRole, flagPrjSyncDryRun)
	},
}

func init() {
	ProjectCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&flagPrjSyncDryRun, "dry-run", false, "Show what would change without writing")
	syncCmd.Flags().StringVar(&flagPrjSyncRole, "role", "", "Role name (required)")
}

// endpoint parsing (name:<...> or folder:<...>)
type prjEndpointKind int

const (
	epUnknown prjEndpointKind = iota
	epName
	epFolder
)

type prjEndpoint struct {
	kind  prjEndpointKind
	value string
}

func parsePrjEndpoint(s string) (prjEndpoint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return prjEndpoint{}, errors.New("empty endpoint")
	}
	if strings.HasPrefix(s, "name:") {
		v := strings.TrimSpace(strings.TrimPrefix(s, "name:"))
		if v == "" {
			return prjEndpoint{}, errors.New("name endpoint missing value")
		}
		return prjEndpoint{kind: epName, value: v}, nil
	}
	if strings.HasPrefix(s, "folder:") {
		v := strings.TrimSpace(strings.TrimPrefix(s, "folder:"))
		if v == "" {
			return prjEndpoint{}, errors.New("folder endpoint missing path")
		}
		if filepath.IsAbs(v) {
			return prjEndpoint{}, errors.New("folder path must be relative")
		}
		return prjEndpoint{kind: epFolder, value: v}, nil
	}
	return prjEndpoint{}, fmt.Errorf("unsupported endpoint %q (use name:<text> or folder:<path>)", s)
}

// YAML rendering helpers (copied from blackboard sync for consistency)
// LiteralString renders YAML as a literal block (|), preserving newlines.
type LiteralString string

func (s LiteralString) MarshalYAML() (any, error) {
	n := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.LiteralStyle, Value: string(s)}
	return &n, nil
}

// ProjectYAML controls YAML output for a single project.
type ProjectYAML struct {
	Name        string         `yaml:"name"`
	Role        string         `yaml:"role"`
	Description *LiteralString `yaml:"description,omitempty"`
	Notes       *LiteralString `yaml:"notes,omitempty"`
	Tags        map[string]any `yaml:"tags,omitempty"`
	Created     *string        `yaml:"created,omitempty"`
	Updated     *string        `yaml:"updated,omitempty"`
}

func syncNameToFolder(projectName, relFolder, role string, dryRun bool) error {
	// Validate and prepare destination directory
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgdao.OpenApp(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	// Fetch project
	p, err := pgdao.GetProjectByKey(ctx, db, projectName, role)
	if err != nil {
		return err
	}

	// Build YAML struct
	y := ProjectYAML{Name: p.Name, Role: p.RoleName, Tags: p.Tags}
	if p.Description.Valid && strings.TrimSpace(p.Description.String) != "" {
		v := LiteralString(p.Description.String)
		y.Description = &v
	}
	if p.Notes.Valid && strings.TrimSpace(p.Notes.String) != "" {
		v := LiteralString(p.Notes.String)
		y.Notes = &v
	}
	if p.Created.Valid {
		v := p.Created.Time.Format(time.RFC3339Nano)
		y.Created = &v
	}
	if p.Updated.Valid {
		v := p.Updated.Time.Format(time.RFC3339Nano)
		y.Updated = &v
	}

	// Compute filename and write
	safe := sanitizeForFile(p.Name)
	if safe == "" {
		// fallback to raw name with non-alnum replaced (should not happen)
		safe = "project"
	}
	outFile := filepath.Join(destDir, fmt.Sprintf("%s.project.yaml", safe))
	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] write %s\n", outFile)
		return nil
	}
	if err := writeYAML(outFile, y); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outFile)
	return nil
}

// writeYAML writes YAML atomically (tmp + rename).
func writeYAML(path string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// sanitizeForFile replaces any rune that is not alphanumeric, '_' or '-' with '-'.
// It also lowercases the result, collapses consecutive '-', and trims leading/trailing '-'.
func sanitizeForFile(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		valid := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
		if !valid {
			r = '-'
		}
		if r == '-' {
			if prevDash {
				continue
			}
			prevDash = true
			b.WriteRune(r)
			continue
		}
		prevDash = false
		b.WriteRune(r)
	}
	out := strings.Trim(b.String(), "-")
	return out
}
