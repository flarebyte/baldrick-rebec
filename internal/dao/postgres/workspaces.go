package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Workspace struct {
	ID            string
	Description   sql.NullString
	RoleName      string
	ProjectName   sql.NullString
	BuildScriptID sql.NullString
	Tags          map[string]any
	Created       sql.NullTime
	Updated       sql.NullTime
}

// UpsertWorkspace inserts a new workspace if ID is empty, otherwise updates it.
func UpsertWorkspace(ctx context.Context, db *pgxpool.Pool, w *Workspace) error {
	if w.ID != "" {
		q := `UPDATE workspaces
              SET description=NULLIF($2,''), role_name=$3, project_name=NULLIF($4,''), tags=COALESCE($5,'{}'::jsonb), build_script_id=CASE WHEN $6='' THEN NULL ELSE $6::uuid END, updated=now()
              WHERE id=$1::uuid
              RETURNING created, updated`
		var tagsJSON []byte
		if w.Tags != nil {
			tagsJSON, _ = json.Marshal(w.Tags)
		}
		if err := db.QueryRow(ctx, q, w.ID, stringOrEmpty(w.Description), w.RoleName, stringOrEmpty(w.ProjectName), tagsJSON, stringOrEmpty(w.BuildScriptID)).Scan(&w.Created, &w.Updated); err != nil {
			return dbutil.ErrWrap("workspace.upsert.update", err, dbutil.ParamSummary("id", w.ID), dbutil.ParamSummary("role", w.RoleName))
		}
		return nil
	}
	q := `INSERT INTO workspaces (description, role_name, project_name, tags, build_script_id)
          VALUES (NULLIF($1,''), $2, NULLIF($3,''), COALESCE($4,'{}'::jsonb), CASE WHEN $5='' THEN NULL ELSE $5::uuid END)
          RETURNING id::text, created, updated`
	var tagsJSON []byte
	if w.Tags != nil {
		tagsJSON, _ = json.Marshal(w.Tags)
	}
	if err := db.QueryRow(ctx, q, stringOrEmpty(w.Description), w.RoleName, stringOrEmpty(w.ProjectName), tagsJSON, stringOrEmpty(w.BuildScriptID)).Scan(&w.ID, &w.Created, &w.Updated); err != nil {
		return dbutil.ErrWrap("workspace.upsert.insert", err, dbutil.ParamSummary("role", w.RoleName))
	}
	return nil
}

// GetWorkspaceByID fetches a workspace by UUID.
func GetWorkspaceByID(ctx context.Context, db *pgxpool.Pool, id string) (*Workspace, error) {
	q := `SELECT id::text, description, role_name, project_name, tags, build_script_id::text, created, updated
          FROM workspaces WHERE id=$1::uuid`
	var w Workspace
	var tagsJSON []byte
	if err := db.QueryRow(ctx, q, id).Scan(&w.ID, &w.Description, &w.RoleName, &w.ProjectName, &tagsJSON, &w.BuildScriptID, &w.Created, &w.Updated); err != nil {
		return nil, dbutil.ErrWrap("workspace.get", err, dbutil.ParamSummary("id", id))
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &w.Tags)
	}
	return &w, nil
}

// ListWorkspaces lists workspaces filtered by role.
func ListWorkspaces(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Workspace, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT id::text, description, role_name, project_name, tags, build_script_id::text, created, updated
          FROM workspaces WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`
	rows, err := db.Query(ctx, q, roleName, limit, offset)
	if err != nil {
		return nil, dbutil.ErrWrap("workspace.list", err, dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		var tagsJSON []byte
		if err := rows.Scan(&w.ID, &w.Description, &w.RoleName, &w.ProjectName, &tagsJSON, &w.BuildScriptID, &w.Created, &w.Updated); err != nil {
			return nil, dbutil.ErrWrap("workspace.list.scan", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &w.Tags)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("workspace.list", err)
	}
	return out, nil
}

// DeleteWorkspace deletes by id.
func DeleteWorkspace(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM workspaces WHERE id=$1::uuid`, id)
	if err != nil {
		return 0, dbutil.ErrWrap("workspace.delete", err, dbutil.ParamSummary("id", id))
	}
	return ct.RowsAffected(), nil
}
