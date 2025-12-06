package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Graph features migrated to SQL tables.

// EnsureTaskVertex is now a no-op.
func EnsureTaskVertex(ctx context.Context, db *pgxpool.Pool, id, variant, command string) error {
	return nil
}

func normalizeStickieRelType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "includes":
		return "INCLUDES"
	case "causes":
		return "CAUSES"
	case "uses":
		return "USES"
	case "represents":
		return "REPRESENTS"
	case "contrasts_with", "contrasts-with", "contrastswith":
		return "CONTRASTS_WITH"
	default:
		return ""
	}
}

// CreateStickieEdge stores/updates relation in SQL mirror table.
func CreateStickieEdge(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string, labels []string) error {
	rt := normalizeStickieRelType(relType)
	if rt == "" {
		return fmt.Errorf("invalid relation type: %s", relType)
	}
	if strings.TrimSpace(fromID) == "" || strings.TrimSpace(toID) == "" {
		return fmt.Errorf("from/to ids required")
	}
	return UpsertStickieRelation(ctx, db, StickieRelation{FromID: fromID, ToID: toID, RelType: rt, Labels: labels})
}

type StickieEdge struct {
	FromID string
	ToID   string
	Type   string
	Labels []string
}

// ListStickieEdges via SQL mirror; supports dir and type filters.
func ListStickieEdges(ctx context.Context, db *pgxpool.Pool, id, dir string, relTypes []string) ([]StickieEdge, error) {
	srels, err := ListStickieRelations(ctx, db, id, dir)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	if len(relTypes) > 0 {
		for _, t := range relTypes {
			if nt := normalizeStickieRelType(t); nt != "" {
				allowed[nt] = true
			}
		}
	}
	out := make([]StickieEdge, 0, len(srels))
	for _, r := range srels {
		if len(allowed) > 0 && !allowed[r.RelType] {
			continue
		}
		out = append(out, StickieEdge{FromID: r.FromID, ToID: r.ToID, Type: r.RelType, Labels: r.Labels})
	}
	return out, nil
}

// GetStickieEdge returns a single relation between exact from/to/type.
func GetStickieEdge(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string) (*StickieEdge, error) {
	rt := normalizeStickieRelType(relType)
	if rt == "" {
		return nil, fmt.Errorf("invalid relation type: %s", relType)
	}
	r, err := GetStickieRelation(ctx, db, fromID, toID, rt)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	return &StickieEdge{FromID: r.FromID, ToID: r.ToID, Type: r.RelType, Labels: r.Labels}, nil
}

// DeleteStickieEdge deletes edges of a type between two stickies and returns affected count.
func DeleteStickieEdge(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string) (int64, error) {
	rt := normalizeStickieRelType(relType)
	if rt == "" {
		return 0, fmt.Errorf("invalid relation type: %s", relType)
	}
	return DeleteStickieRelation(ctx, db, fromID, toID, rt)
}

// SQL implementations for task relation queries
func CreateTaskReplacesEdgeSQL(ctx context.Context, db *pgxpool.Pool, newTaskID, oldTaskID, level, comment, createdISO string) error {
	return CreateTaskReplacesEdge(ctx, db, newTaskID, oldTaskID, level, comment, createdISO)
}

// CreateTaskReplacesEdge stores a replacement relation in SQL.
func CreateTaskReplacesEdge(ctx context.Context, db *pgxpool.Pool, newTaskID, oldTaskID, level, comment, createdISO string) error {
	if strings.TrimSpace(newTaskID) == "" || strings.TrimSpace(oldTaskID) == "" {
		return fmt.Errorf("task ids required")
	}
	lvl := strings.ToLower(strings.TrimSpace(level))
	switch lvl {
	case "patch", "minor", "major":
	default:
		lvl = "minor"
	}
	if strings.TrimSpace(createdISO) == "" {
		_, err := db.Exec(ctx, `INSERT INTO task_replaces (new_task_id, old_task_id, level, comment)
                                 VALUES ($1::uuid,$2::uuid,$3,$4)
                                 ON CONFLICT (new_task_id,old_task_id) DO UPDATE SET level=EXCLUDED.level, comment=EXCLUDED.comment`,
			newTaskID, oldTaskID, lvl, comment)
		return err
	}
	_, err := db.Exec(ctx, `INSERT INTO task_replaces (new_task_id, old_task_id, level, comment, created)
                             VALUES ($1::uuid,$2::uuid,$3,$4,$5::timestamptz)
                             ON CONFLICT (new_task_id,old_task_id) DO UPDATE SET level=EXCLUDED.level, comment=EXCLUDED.comment, created=EXCLUDED.created`,
		newTaskID, oldTaskID, lvl, comment, createdISO)
	return err
}

// FindLatestTaskIDByVariant returns a task with the given variant that has no incoming replacement.
func FindLatestTaskIDByVariant(ctx context.Context, db *pgxpool.Pool, variant string) (string, error) {
	q := `SELECT t.id::text
          FROM tasks t
          WHERE t.variant=$1
            AND NOT EXISTS (SELECT 1 FROM task_replaces tr WHERE tr.old_task_id=t.id)
          ORDER BY t.created DESC
          LIMIT 1`
	var id string
	if err := db.QueryRow(ctx, q, variant).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// FindNextByLevel returns the direct successor of current with the given level.
func FindNextByLevel(ctx context.Context, db *pgxpool.Pool, currentID, level string) (string, error) {
	lvl := strings.ToLower(strings.TrimSpace(level))
	if lvl != "patch" && lvl != "minor" && lvl != "major" {
		return "", fmt.Errorf("invalid level: %s", level)
	}
	q := `SELECT tr.new_task_id::text
          FROM task_replaces tr
          WHERE tr.old_task_id=$1::uuid AND tr.level=$2
          ORDER BY tr.created DESC
          LIMIT 1`
	var id string
	if err := db.QueryRow(ctx, q, currentID, lvl).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// FindLatestFrom returns the latest Task reachable via REPLACES* from current.
func FindLatestFrom(ctx context.Context, db *pgxpool.Pool, currentID string) (string, error) {
	q := `WITH RECURSIVE succ(id) AS (
              SELECT new_task_id FROM task_replaces WHERE old_task_id=$1::uuid
            UNION ALL
              SELECT tr.new_task_id FROM task_replaces tr JOIN succ s ON tr.old_task_id=s.id
          )
          SELECT s.id::text
          FROM succ s
          WHERE NOT EXISTS (SELECT 1 FROM task_replaces x WHERE x.old_task_id=s.id)
          ORDER BY s.id
          LIMIT 1`
	var id string
	if err := db.QueryRow(ctx, q, currentID).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}
