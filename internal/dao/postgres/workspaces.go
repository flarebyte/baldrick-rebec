package postgres

import (
    "context"
    "database/sql"
    "encoding/json"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Workspace struct {
    ID          string
    Description sql.NullString
    RoleName    string
    ProjectName sql.NullString
    Tags        map[string]any
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertWorkspace inserts a new workspace if ID is empty, otherwise updates it.
func UpsertWorkspace(ctx context.Context, db *pgxpool.Pool, w *Workspace) error {
    if w.ID != "" {
        q := `UPDATE workspaces
              SET description=NULLIF($2,''), role_name=$3, project_name=NULLIF($4,''), tags=COALESCE($5,'{}'::jsonb), updated=now()
              WHERE id=$1::uuid
              RETURNING created, updated`
        var tagsJSON []byte
        if w.Tags != nil { tagsJSON, _ = json.Marshal(w.Tags) }
        return db.QueryRow(ctx, q, w.ID, stringOrEmpty(w.Description), w.RoleName, stringOrEmpty(w.ProjectName), tagsJSON).Scan(&w.Created, &w.Updated)
    }
    q := `INSERT INTO workspaces (description, role_name, project_name, tags)
          VALUES (NULLIF($1,''), $2, NULLIF($3,''), COALESCE($4,'{}'::jsonb))
          RETURNING id::text, created, updated`
    var tagsJSON []byte
    if w.Tags != nil { tagsJSON, _ = json.Marshal(w.Tags) }
    return db.QueryRow(ctx, q, stringOrEmpty(w.Description), w.RoleName, stringOrEmpty(w.ProjectName), tagsJSON).Scan(&w.ID, &w.Created, &w.Updated)
}

// GetWorkspaceByID fetches a workspace by UUID.
func GetWorkspaceByID(ctx context.Context, db *pgxpool.Pool, id string) (*Workspace, error) {
    q := `SELECT id::text, description, role_name, project_name, tags, created, updated
          FROM workspaces WHERE id=$1::uuid`
    var w Workspace
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, id).Scan(&w.ID, &w.Description, &w.RoleName, &w.ProjectName, &tagsJSON, &w.Created, &w.Updated); err != nil {
        return nil, err
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &w.Tags) }
    return &w, nil
}

// ListWorkspaces lists workspaces filtered by role.
func ListWorkspaces(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Workspace, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT id::text, description, role_name, project_name, tags, created, updated
          FROM workspaces WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Workspace
    for rows.Next() {
        var w Workspace
        var tagsJSON []byte
        if err := rows.Scan(&w.ID, &w.Description, &w.RoleName, &w.ProjectName, &tagsJSON, &w.Created, &w.Updated); err != nil {
            return nil, err
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &w.Tags) }
        out = append(out, w)
    }
    return out, rows.Err()
}

// DeleteWorkspace deletes by id.
func DeleteWorkspace(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM workspaces WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
