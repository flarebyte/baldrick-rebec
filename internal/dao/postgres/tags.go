package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Tag struct {
    Name        string
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertTag inserts or updates a tag by name.
func UpsertTag(ctx context.Context, db *pgxpool.Pool, t *Tag) error {
    q := `INSERT INTO tags (name, title, description, notes)
          VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))
          ON CONFLICT (name) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            notes = EXCLUDED.notes,
            updated = now()
          RETURNING created, updated`
    return db.QueryRow(ctx, q, t.Name, t.Title, stringOrEmpty(t.Description), stringOrEmpty(t.Notes)).Scan(&t.Created, &t.Updated)
}

// GetTagByName fetches a tag by its unique name.
func GetTagByName(ctx context.Context, db *pgxpool.Pool, name string) (*Tag, error) {
    q := `SELECT name, title, description, notes, created, updated FROM tags WHERE name=$1`
    var t Tag
    if err := db.QueryRow(ctx, q, name).Scan(&t.Name, &t.Title, &t.Description, &t.Notes, &t.Created, &t.Updated); err != nil {
        return nil, err
    }
    return &t, nil
}

// ListTags returns tags ordered by name with optional pagination and required role filter.
func ListTags(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Tag, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT name, title, description, notes, created, updated
          FROM tags
          WHERE role_name=$1
          ORDER BY name ASC
          LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Tag
    for rows.Next() {
        var t Tag
        if err := rows.Scan(&t.Name, &t.Title, &t.Description, &t.Notes, &t.Created, &t.Updated); err != nil {
            return nil, err
        }
        out = append(out, t)
    }
    return out, rows.Err()
}

// DeleteTag removes a tag by name. Returns number of rows affected.
func DeleteTag(ctx context.Context, db *pgxpool.Pool, name string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM tags WHERE name=$1`, name)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

