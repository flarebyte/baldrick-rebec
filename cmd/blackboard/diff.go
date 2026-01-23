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
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

var (
	flagDiffDetailed        bool
	flagDiffIncludeArchived bool
)

// diffCmd implements: rbc blackboard diff id:UUID folder:relative/path (or reverse)
var diffCmd = &cobra.Command{
	Use:   "diff <left> <right>",
	Short: "Diff a blackboard (id) against a folder",
	Long:  "Compare a remote blackboard (id:UUID) with a local folder (folder:RELATIVE_PATH) and display concise or detailed differences.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		left, err := parseEndpoint(args[0])
		if err != nil {
			return err
		}
		right, err := parseEndpoint(args[1])
		if err != nil {
			return err
		}
		if left.kind == right.kind {
			return fmt.Errorf("cannot diff %s vs %s; use id:UUID and folder:PATH", kindString(left.kind), kindString(right.kind))
		}

		// Normalize so that remote is the id endpoint and local is the folder endpoint
		var idEp endpoint
		var folderEp endpoint
		if left.kind == epID {
			idEp = left
			folderEp = right
		} else {
			idEp = right
			folderEp = left
		}

		// Shortcut: allow id:_ to read id from the folder's blackboard.yaml
		if idEp.value == "_" {
			bbid, e := readBlackboardIDFromFolder(folderEp.value)
			if e != nil {
				return e
			}
			idEp.value = bbid
		}

		return runBlackboardDiff(idEp.value, folderEp.value, flagDiffDetailed, flagDiffIncludeArchived)
	},
}

func init() {
	BlackboardCmd.AddCommand(diffCmd)
	diffCmd.Flags().BoolVar(&flagDiffDetailed, "detailed", false, "Show detailed differences (default: concise)")
	diffCmd.Flags().BoolVar(&flagDiffIncludeArchived, "include-archived", false, "Include archived stickies in diff (default: active only)")
}

// runBlackboardDiff performs the diff between remote id and local folder.
func runBlackboardDiff(blackboardID, relFolder string, detailed, includeArchived bool) error {
	// Validate folder
	folder := filepath.Clean(relFolder)
	if strings.HasPrefix(folder, "..") {
		return errors.New("folder path must not escape current directory")
	}

	// Load local blackboard.yaml (optional but recommended)
	var localBB blackboardYAML
	var localBBErr error
	if bby, err := readLocalBlackboardYAML(filepath.Join(folder, "blackboard.yaml")); err != nil {
		localBBErr = err
	} else {
		localBB = bby
	}

	// Open DB
	cfg, err := cfgpkg.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := pgdao.OpenApp(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

    // Load remote blackboard but do not fail hard if missing
    var rb *pgdao.Blackboard
    if b, e := pgdao.GetBlackboardByID(ctx, db, blackboardID); e == nil {
        rb = b
    } else {
        // Keep going to provide a deep diff on local contents
        rb = nil
    }

    // Present blackboard diff (handles nil remote or missing local)
    printBlackboardDiff(rb, localBB, localBBErr, detailed)

    // Load remote stickies (paged) only if the blackboard exists in DB
    remoteStickies := make([]pgdao.Stickie, 0, 256)
    if rb != nil {
        const page = 1000
        for off := 0; ; off += page {
            ss, err := pgdao.ListStickies(ctx, db, blackboardID, page, off)
            if err != nil {
                return err
            }
            remoteStickies = append(remoteStickies, ss...)
            if len(ss) < page {
                break
            }
        }
    }
	// Filter archived optionally
	if !includeArchived {
		filtered := remoteStickies[:0]
		for _, s := range remoteStickies {
			if !s.Archived {
				filtered = append(filtered, s)
			}
		}
		remoteStickies = filtered
	}

	// Load local stickies
	localByID, localAnon, err := loadLocalStickies(folder, includeArchived)
	if err != nil {
		return err
	}

	// Build map for remote
	remoteByID := make(map[string]pgdao.Stickie, len(remoteStickies))
	ids := make([]string, 0, len(remoteStickies))
	for _, s := range remoteStickies {
		remoteByID[s.ID] = s
		ids = append(ids, s.ID)
	}
	sort.Strings(ids)

	// For deterministic local-only ordering, collect local IDs and filenames
	localIDs := make([]string, 0, len(localByID))
	for id := range localByID {
		localIDs = append(localIDs, id)
	}
	sort.Strings(localIDs)
	sort.Slice(localAnon, func(i, j int) bool { return localAnon[i].filename < localAnon[j].filename })

	// Compare per stickie present remotely
	for _, id := range ids {
		rs := remoteByID[id]
		lname := ""
		if rs.Name.Valid {
			lname = rs.Name.String
		}
		if ls, ok := localByID[id]; ok {
			// Present on both sides
			if hashStickieDB(rs) == hashStickieYAML(ls.yaml) {
				// Unchanged
				fmt.Fprintf(os.Stdout, "= stickie id=%s name=%q\n", id, lname)
				continue
			}
			// Changed
			if detailed {
				fields := computeStickieFieldDiff(rs, ls.yaml)
				fmt.Fprintf(os.Stdout, "~ stickie id=%s name=%q changed:%s\n", id, lname, formatDetailedFieldDiff(fields))
			} else {
				fields := listChangedStickieFields(rs, ls.yaml)
				fmt.Fprintf(os.Stdout, "~ stickie id=%s name=%q fields:%s\n", id, lname, strings.Join(fields, ","))
			}
		} else {
			// Remote only
			fmt.Fprintf(os.Stdout, "+ stickie id=%s name=%q (remote-only)\n", id, lname)
		}
	}

    // Local-only stickies (with id). Also check if they belong to a different board in DB.
    for _, id := range localIDs {
        if _, ok := remoteByID[id]; ok {
            continue
        }
        ls := localByID[id]
        lname := ""
        if ls.yaml.Name != nil {
            lname = *ls.yaml.Name
        }
        // Default annotation
        note := "local-only"
        // Try to resolve in DB to see if it belongs to any board
        if s, e := pgdao.GetStickieByID(ctx, db, id); e == nil && s != nil {
            // If it belongs to a different board, show that id
            if strings.TrimSpace(s.BlackboardID) != strings.TrimSpace(blackboardID) {
                note = fmt.Sprintf("local-only, belongs to board %s", s.BlackboardID)
            }
        }
        fmt.Fprintf(os.Stdout, "- stickie id=%s name=%q file=%s (%s)\n", id, lname, ls.filename, note)
    }
	// Local-only stickies without id
	for _, lc := range localAnon {
		lname := ""
		if lc.yaml.Name != nil {
			lname = *lc.yaml.Name
		}
		fmt.Fprintf(os.Stdout, "- stickie name=%q file=%s (local-only, no id)\n", lname, lc.filename)
	}

	return nil
}

// Helpers for blackboard diff
func readLocalBlackboardYAML(path string) (blackboardYAML, error) {
	var y blackboardYAML
	b, err := os.ReadFile(path)
	if err != nil {
		return y, err
	}
	if err := yamlUnmarshalCompat(b, &y); err != nil {
		return y, err
	}
	return y, nil
}

// yamlUnmarshalCompat proxies to yaml.Unmarshal from sync.go’s imported module without reimport noise.
func yamlUnmarshalCompat(b []byte, v any) error {
	return yaml.Unmarshal(b, v)
}

func printBlackboardDiff(remote *pgdao.Blackboard, local blackboardYAML, localErr error, detailed bool) {
    // Remote-only
    if remote != nil && localErr != nil {
        fmt.Fprintf(os.Stdout, "+ blackboard id=%s role=%q (remote-only: no local blackboard.yaml)\n", remote.ID, remote.RoleName)
        return
    }
    // Local-only
    if remote == nil && localErr == nil {
        rid := strings.TrimSpace(local.ID)
        fmt.Fprintf(os.Stdout, "- blackboard id=%s role=%q (local-only)\n", rid, local.Role)
        return
    }
    if remote == nil && localErr != nil {
        // Neither side present; nothing to compare
        fmt.Fprintln(os.Stdout, "- blackboard (local missing and remote missing)")
        return
    }
	// Compare selected fields (ignore timestamps)
	changed := make([]string, 0, 8)
	det := make(map[string][2]string)

	cmp := func(name string, r, l string) {
		if strings.TrimSpace(r) != strings.TrimSpace(l) {
			changed = append(changed, name)
			det[name] = [2]string{r, l}
		}
	}

	// role
	cmp("role", remote.RoleName, local.Role)
	// conversation_id
	cmp("conversation_id", nullOrString(remote.ConversationID), deref(local.Conversation))
	// project
	cmp("project", nullOrString(remote.ProjectName), deref(local.Project))
	// task_id
	cmp("task_id", nullOrString(remote.TaskID), deref(local.TaskID))
	// background (export wraps at 80)
	cmp("background", wrapIfValid(remote.Background), derefLS(local.Background))
	// guidelines (export wraps at 80)
	cmp("guidelines", wrapIfValid(remote.Guidelines), derefLS(local.Guidelines))
	// lifecycle
	cmp("lifecycle", nullOrString(remote.Lifecycle), deref(local.Lifecycle))

	if len(changed) == 0 {
		fmt.Fprintf(os.Stdout, "= blackboard id=%s role=%q\n", remote.ID, remote.RoleName)
		return
	}
	if !detailed {
		fmt.Fprintf(os.Stdout, "~ blackboard id=%s role=%q fields:%s\n", remote.ID, remote.RoleName, strings.Join(changed, ","))
		return
	}
	// Detailed: print per-field values
	var b strings.Builder
	for _, f := range changed {
		v := det[f]
		b.WriteString(" ")
		b.WriteString(f)
		b.WriteString("[remote=")
		b.WriteString(quoteShort(v[0]))
		b.WriteString(" local=")
		b.WriteString(quoteShort(v[1]))
		b.WriteString("]")
	}
	fmt.Fprintf(os.Stdout, "~ blackboard id=%s role=%q:%s\n", remote.ID, remote.RoleName, b.String())
}

// Helpers to load local stickies
type localStickie struct {
	filename string
	yaml     stickieYAML
}

func loadLocalStickies(folder string, includeArchived bool) (map[string]localStickie, []localStickie, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, nil, fmt.Errorf("read folder: %w", err)
	}
	byID := make(map[string]localStickie)
	var anon []localStickie
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".stickie.yaml") {
			continue
		}
		p := filepath.Join(folder, e.Name())
		y, err := readStickieYAML(p)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", p, err)
		}
		if !includeArchived && y.Archived {
			continue
		}
		rec := localStickie{filename: e.Name(), yaml: y}
		id := strings.TrimSpace(y.ID)
		if id == "" {
			anon = append(anon, rec)
		} else {
			byID[id] = rec
		}
	}
	return byID, anon, nil
}

// Concise list of changed fields for a stickie
