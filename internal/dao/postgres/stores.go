package postgres

import (
    "context"
    "database/sql"
    "encoding/json"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
    ID         string
    Name       string
    Title      string
    Description sql.NullString
    Motivation sql.NullString
    Security   sql.NullString
    Privacy    sql.NullString
    RoleName   string
    Notes      sql.NullString
    Tags       map[string]any
    StoreType  sql.NullString
    Scope      sql.NullString
    Lifecycle  sql.NullString
    Created    sql.NullTime
    Updated    sql.NullTime
}

// UpsertStore inserts or updates a store identified by (name, role_name).
// Returns ID for both insert and update.
func UpsertStore(ctx context.Context, db *pgxpool.Pool, s *Store) error {
    q := `INSERT INTO stores (
            name, title, description, motivation, security, privacy,
            role_name, notes, tags, store_type, scope, lifecycle
          ) VALUES (
            $1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''),
            $7, NULLIF($8,''), COALESCE($9,'{}'::jsonb), NULLIF($10,''), NULLIF($11,''), NULLIF($12,'')
          )
          ON CONFLICT (name, role_name) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            motivation = EXCLUDED.motivation,
            security = EXCLUDED.security,
            privacy = EXCLUDED.privacy,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            store_type = EXCLUDED.store_type,
            scope = EXCLUDED.scope,
            lifecycle = EXCLUDED.lifecycle,
            updated = now()
          RETURNING id::text, created, updated`
    var tagsJSON []byte
    if s.Tags != nil { tagsJSON, _ = json.Marshal(s.Tags) }
    return db.QueryRow(ctx, q,
        s.Name, s.Title, stringOrEmpty(s.Description), stringOrEmpty(s.Motivation), stringOrEmpty(s.Security), stringOrEmpty(s.Privacy),
        s.RoleName, stringOrEmpty(s.Notes), tagsJSON, stringOrEmpty(s.StoreType), stringOrEmpty(s.Scope), stringOrEmpty(s.Lifecycle),
    ).Scan(&s.ID, &s.Created, &s.Updated)
}

// GetStoreByKey fetches a store by (name, role_name).
func GetStoreByKey(ctx context.Context, db *pgxpool.Pool, name, roleName string) (*Store, error) {
    q := `SELECT id::text, name, title, description, motivation, security, privacy,
                 role_name, notes, tags, store_type, scope, lifecycle, created, updated
          FROM stores WHERE name=$1 AND role_name=$2`
    var s Store
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, name, roleName).Scan(
        &s.ID, &s.Name, &s.Title, &s.Description, &s.Motivation, &s.Security, &s.Privacy,
        &s.RoleName, &s.Notes, &tagsJSON, &s.StoreType, &s.Scope, &s.Lifecycle, &s.Created, &s.Updated,
    ); err != nil {
        return nil, err
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
    return &s, nil
}

// ListStores returns stores for a role ordered by updated desc, created desc.
func ListStores(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Store, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT id::text, name, title, description, motivation, security, privacy,
                 role_name, notes, tags, store_type, scope, lifecycle, created, updated
          FROM stores WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Store
    for rows.Next() {
        var s Store
        var tagsJSON []byte
        if err := rows.Scan(
            &s.ID, &s.Name, &s.Title, &s.Description, &s.Motivation, &s.Security, &s.Privacy,
            &s.RoleName, &s.Notes, &tagsJSON, &s.StoreType, &s.Scope, &s.Lifecycle, &s.Created, &s.Updated,
        ); err != nil {
            return nil, err
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
        out = append(out, s)
    }
    return out, rows.Err()
}

// DeleteStore removes a store by (name, role_name).
func DeleteStore(ctx context.Context, db *pgxpool.Pool, name, roleName string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM stores WHERE name=$1 AND role_name=$2`, name, roleName)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

