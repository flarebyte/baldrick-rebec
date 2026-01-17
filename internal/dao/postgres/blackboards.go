package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Blackboard struct {
	ID             string
	RoleName       string
	ConversationID sql.NullString
	ProjectName    sql.NullString
	TaskID         sql.NullString
	Background     sql.NullString
	Guidelines     sql.NullString
	Lifecycle      sql.NullString
	Created        sql.NullTime
	Updated        sql.NullTime
}

// BlackboardWithRefs flattens blackboard plus selected display fields from joined tables.
// Related fields are prefixed with their table name for clarity.
type BlackboardWithRefs struct {
	// Base blackboard fields
	ID             string
	RoleName       string
	ConversationID sql.NullString
	ProjectName    sql.NullString
	TaskID         sql.NullString
	Background     sql.NullString
	Guidelines     sql.NullString
	Lifecycle      sql.NullString
	Created        sql.NullTime
	Updated        sql.NullTime

	// Related display fields (all optional)
	TaskVariant       sql.NullString // tasks.variant
	TaskTitle         sql.NullString // tasks.title
	ProjectDesc       sql.NullString // projects.description
	ProjectNotes      sql.NullString // projects.notes
	ConversationTitle sql.NullString // conversations.title
}

// UpsertBlackboard inserts a new blackboard if ID is empty, otherwise updates it.
func UpsertBlackboard(ctx context.Context, db *pgxpool.Pool, b *Blackboard) error {
	if b.ID != "" {
		q := `UPDATE blackboards
              SET role_name=$2,
                  conversation_id=CASE WHEN $3='' THEN NULL ELSE $3::uuid END,
                  project_name=NULLIF($4,''),
                  task_id=CASE WHEN $5='' THEN NULL ELSE $5::uuid END,
                  background=NULLIF($6,''),
                  guidelines=NULLIF($7,''),
                  lifecycle=NULLIF($8,''),
                  updated=now()
              WHERE id=$1::uuid
              RETURNING created, updated`
		if err := db.QueryRow(ctx, q,
			b.ID, b.RoleName,
			stringOrEmpty(b.ConversationID), stringOrEmpty(b.ProjectName), stringOrEmpty(b.TaskID),
			stringOrEmpty(b.Background), stringOrEmpty(b.Guidelines), stringOrEmpty(b.Lifecycle),
		).Scan(&b.Created, &b.Updated); err != nil {
			return dbutil.ErrWrap("blackboard.upsert.update", err,
				dbutil.ParamSummary("id", b.ID), dbutil.ParamSummary("role", b.RoleName))
		}
		return nil
	}
	q := `INSERT INTO blackboards (role_name, conversation_id, project_name, task_id, background, guidelines, lifecycle)
          VALUES ($1, CASE WHEN $2='' THEN NULL ELSE $2::uuid END, NULLIF($3,''), CASE WHEN $4='' THEN NULL ELSE $4::uuid END, NULLIF($5,''), NULLIF($6,''), NULLIF($7,''))
          RETURNING id::text, created, updated`
	if err := db.QueryRow(ctx, q,
		b.RoleName, stringOrEmpty(b.ConversationID), stringOrEmpty(b.ProjectName), stringOrEmpty(b.TaskID), stringOrEmpty(b.Background), stringOrEmpty(b.Guidelines), stringOrEmpty(b.Lifecycle),
	).Scan(&b.ID, &b.Created, &b.Updated); err != nil {
		return dbutil.ErrWrap("blackboard.upsert.insert", err,
			dbutil.ParamSummary("role", b.RoleName))
	}
	return nil
}

// GetBlackboardByID fetches a blackboard by UUID.
func GetBlackboardByID(ctx context.Context, db *pgxpool.Pool, id string) (*Blackboard, error) {
	q := `SELECT id::text, role_name, conversation_id::text, project_name, task_id::text, background, guidelines, lifecycle, created, updated
          FROM blackboards WHERE id=$1::uuid`
	var b Blackboard
	if err := db.QueryRow(ctx, q, id).Scan(&b.ID, &b.RoleName, &b.ConversationID, &b.ProjectName, &b.TaskID, &b.Background, &b.Guidelines, &b.Lifecycle, &b.Created, &b.Updated); err != nil {
		return nil, dbutil.ErrWrap("blackboard.get", err, dbutil.ParamSummary("id", id))
	}
	return &b, nil
}

// ListBlackboards lists blackboards filtered by role.
func ListBlackboards(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Blackboard, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT id::text, role_name, conversation_id::text, project_name, task_id::text, background, guidelines, lifecycle, created, updated
          FROM blackboards WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`
	rows, err := db.Query(ctx, q, roleName, limit, offset)
	if err != nil {
		return nil, dbutil.ErrWrap("blackboard.list", err,
			dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []Blackboard
	for rows.Next() {
		var b Blackboard
		if err := rows.Scan(&b.ID, &b.RoleName, &b.ConversationID, &b.ProjectName, &b.TaskID, &b.Background, &b.Guidelines, &b.Lifecycle, &b.Created, &b.Updated); err != nil {
			return nil, dbutil.ErrWrap("blackboard.list.scan", err, dbutil.ParamSummary("role", roleName))
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("blackboard.list", err, dbutil.ParamSummary("role", roleName))
	}
	return out, nil
}

// ListBlackboardsWithRefs returns recent blackboards for a role and joins display fields
// from stores, tasks, projects, and conversations. Fields are returned flattened
// with prefixes as documented in BlackboardWithRefs.
func ListBlackboardsWithRefs(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int, search string) ([]BlackboardWithRefs, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	// Base SELECT with joins
	base := `SELECT 
            b.id::text, b.role_name,
            b.conversation_id::text, b.project_name, b.task_id::text,
            b.background, b.guidelines, b.lifecycle, b.created, b.updated,
            t.variant, t.title,
            p.description, p.notes,
            c.title
          FROM blackboards b
          LEFT JOIN tasks t ON t.id = b.task_id
          LEFT JOIN projects p ON p.name = b.project_name AND p.role_name = b.role_name
          LEFT JOIN conversations c ON c.id = b.conversation_id
          WHERE b.role_name=$1`
	var rows pgxRows
	var err error
	if strings.TrimSpace(search) != "" {
		// Case-insensitive pattern search across selected related fields
		q := base + ` AND (
              p.name ILIKE $2 OR p.description ILIKE $2 OR b.background ILIKE $2 OR b.guidelines ILIKE $2
            )
            ORDER BY b.updated DESC, b.created DESC
            LIMIT $3 OFFSET $4`
		pattern := "%" + strings.TrimSpace(search) + "%"
		rows, err = db.Query(ctx, q, roleName, pattern, limit, offset)
	} else {
		q := base + `
            ORDER BY b.updated DESC, b.created DESC
            LIMIT $2 OFFSET $3`
		rows, err = db.Query(ctx, q, roleName, limit, offset)
	}
	if err != nil {
		return nil, dbutil.ErrWrap("blackboard.list_refs", err, dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []BlackboardWithRefs
	for rows.Next() {
		var r BlackboardWithRefs
		if err := rows.Scan(
			&r.ID, &r.RoleName,
			&r.ConversationID, &r.ProjectName, &r.TaskID,
			&r.Background, &r.Guidelines, &r.Lifecycle, &r.Created, &r.Updated,
			&r.TaskVariant, &r.TaskTitle,
			&r.ProjectDesc, &r.ProjectNotes,
			&r.ConversationTitle,
		); err != nil {
			return nil, dbutil.ErrWrap("blackboard.list_refs.scan", err, dbutil.ParamSummary("role", roleName))
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("blackboard.list_refs", err, dbutil.ParamSummary("role", roleName))
	}
	return out, nil
}

// DeleteBlackboard deletes by id.
func DeleteBlackboard(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM blackboards WHERE id=$1::uuid`, id)
	if err != nil {
		return 0, dbutil.ErrWrap("blackboard.delete", err, dbutil.ParamSummary("id", id))
	}
	return ct.RowsAffected(), nil
}
