package postgres

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
)

type Topic struct {
    Name        string
    RoleName    string
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Tags        map[string]any
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertTopic inserts or updates a topic identified by (name, role_name).
func UpsertTopic(ctx context.Context, db *pgxpool.Pool, t *Topic) error {
    q := `INSERT INTO topics (name, role_name, title, description, notes, tags)
          VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), COALESCE($6,'{}'::jsonb))
          ON CONFLICT (name, role_name) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            updated = now()
          RETURNING created, updated`
    var tagsJSON []byte
    if t.Tags != nil { tagsJSON, _ = json.Marshal(t.Tags) }
    if err := db.QueryRow(ctx, q, t.Name, t.RoleName, t.Title, stringOrEmpty(t.Description), stringOrEmpty(t.Notes), tagsJSON).Scan(&t.Created, &t.Updated); err != nil {
        return dbutil.ErrWrap("topic.upsert", err, dbutil.ParamSummary("name", t.Name), dbutil.ParamSummary("role", t.RoleName))
    }
    return nil
}

// GetTopicByKey fetches a topic by (name, role_name).
func GetTopicByKey(ctx context.Context, db *pgxpool.Pool, name, roleName string) (*Topic, error) {
    q := `SELECT name, role_name, title, description, notes, tags, created, updated FROM topics WHERE name=$1 AND role_name=$2`
    var t Topic
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, name, roleName).Scan(&t.Name, &t.RoleName, &t.Title, &t.Description, &t.Notes, &tagsJSON, &t.Created, &t.Updated); err != nil {
        return nil, dbutil.ErrWrap("topic.get", err, dbutil.ParamSummary("name", name), dbutil.ParamSummary("role", roleName))
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
    return &t, nil
}

// ListTopics returns topics for a role ordered by name.
func ListTopics(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Topic, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT name, role_name, title, description, notes, tags, created, updated FROM topics WHERE role_name=$1 ORDER BY name ASC LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, dbutil.ErrWrap("topic.list", err, dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset)) }
    defer rows.Close()
    var out []Topic
    for rows.Next() {
        var t Topic
        var tagsJSON []byte
        if err := rows.Scan(&t.Name, &t.RoleName, &t.Title, &t.Description, &t.Notes, &tagsJSON, &t.Created, &t.Updated); err != nil {
            return nil, dbutil.ErrWrap("topic.list.scan", err)
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
        out = append(out, t)
    }
    if err := rows.Err(); err != nil { return nil, dbutil.ErrWrap("topic.list", err) }
    return out, nil
}

// DeleteTopic removes a topic by (name, role_name).
func DeleteTopic(ctx context.Context, db *pgxpool.Pool, name, roleName string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM topics WHERE name=$1 AND role_name=$2`, name, roleName)
    if err != nil { return 0, dbutil.ErrWrap("topic.delete", err, dbutil.ParamSummary("name", name), dbutil.ParamSummary("role", roleName)) }
    return ct.RowsAffected(), nil
}
