package postgres

import (
    "context"
    "database/sql"
    "encoding/json"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Role struct {
    Name        string
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Tags        map[string]any
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertRole creates or updates a role by name.
func UpsertRole(ctx context.Context, db *pgxpool.Pool, r *Role) error {
    q := `INSERT INTO roles (name, title, description, notes, tags)
          VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), COALESCE($5,'{}'::jsonb))
          ON CONFLICT (name) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            updated = now()
          RETURNING created, updated`
    var tagsJSON []byte
    if r.Tags != nil { tagsJSON, _ = json.Marshal(r.Tags) }
    return db.QueryRow(ctx, q,
        r.Name, r.Title, stringOrEmpty(r.Description), stringOrEmpty(r.Notes), tagsJSON,
    ).Scan(&r.Created, &r.Updated)
}

// GetRoleByName fetches a role by name.
func GetRoleByName(ctx context.Context, db *pgxpool.Pool, name string) (*Role, error) {
    q := `SELECT name, title, description, notes, tags, created, updated FROM roles WHERE name=$1`
    var r Role
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, name).Scan(&r.Name, &r.Title, &r.Description, &r.Notes, &tagsJSON, &r.Created, &r.Updated); err != nil {
        return nil, err
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &r.Tags) }
    return &r, nil
}

// ListRoles returns roles ordered by name with pagination.
func ListRoles(ctx context.Context, db *pgxpool.Pool, limit, offset int) ([]Role, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT name, title, description, notes, tags, created, updated FROM roles ORDER BY name LIMIT $1 OFFSET $2`
    rows, err := db.Query(ctx, q, limit, offset)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Role
    for rows.Next() {
        var r Role
        var tagsJSON []byte
        if err := rows.Scan(&r.Name, &r.Title, &r.Description, &r.Notes, &tagsJSON, &r.Created, &r.Updated); err != nil {
            return nil, err
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &r.Tags) }
        out = append(out, r)
    }
    return out, rows.Err()
}

// DeleteRole removes a role by name.
func DeleteRole(ctx context.Context, db *pgxpool.Pool, name string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM roles WHERE name=$1`, name)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
