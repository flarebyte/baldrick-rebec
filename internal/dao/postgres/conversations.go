package postgres

import (
    "context"
    "database/sql"
    "encoding/json"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Conversation struct {
    ID          string
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Project     sql.NullString
    Tags        map[string]any
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertConversation inserts a new conversation if ID==0, otherwise updates the existing one.
func UpsertConversation(ctx context.Context, db *pgxpool.Pool, c *Conversation) error {
    if strings.TrimSpace(c.ID) != "" {
        q := `UPDATE conversations
              SET title=$2, description=NULLIF($3,''), project=NULLIF($4,''), tags=$5::text[], notes=NULLIF($6,''), updated=now()
              WHERE id=$1::uuid RETURNING created, updated`
        return db.QueryRow(ctx, q, c.ID, c.Title, stringOrEmpty(c.Description), stringOrEmpty(c.Project), c.Tags, stringOrEmpty(c.Notes)).Scan(&c.Created, &c.Updated)
    }
    q := `INSERT INTO conversations (title, description, project, tags, notes)
          VALUES ($1, NULLIF($2,''), NULLIF($3,''), COALESCE($4,'{}'::jsonb), NULLIF($5,''))
          RETURNING id::text, created, updated`
    var tagsJSON []byte
    if c.Tags != nil { tagsJSON, _ = json.Marshal(c.Tags) }
    return db.QueryRow(ctx, q, c.Title, stringOrEmpty(c.Description), stringOrEmpty(c.Project), tagsJSON, stringOrEmpty(c.Notes)).Scan(&c.ID, &c.Created, &c.Updated)
}

// GetConversationByID returns a conversation by its id.
func GetConversationByID(ctx context.Context, db *pgxpool.Pool, id string) (*Conversation, error) {
    q := `SELECT id::text, title, description, project, tags, notes, created, updated FROM conversations WHERE id=$1::uuid`
    var c Conversation
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, id).Scan(&c.ID, &c.Title, &c.Description, &c.Project, &tagsJSON, &c.Notes, &c.Created, &c.Updated); err != nil {
        return nil, err
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &c.Tags) }
    return &c, nil
}

// ListConversations lists conversations, optionally filtered by project, with pagination.
func ListConversations(ctx context.Context, db *pgxpool.Pool, project, roleName string, limit, offset int) ([]Conversation, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    if strings.TrimSpace(project) == "" {
        rows, err = db.Query(ctx, `SELECT id::text, title, description, project, tags, notes, created, updated FROM conversations WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`, roleName, limit, offset)
    } else {
        rows, err = db.Query(ctx, `SELECT id::text, title, description, project, tags, notes, created, updated FROM conversations WHERE project=$1 AND role_name=$2 ORDER BY updated DESC, created DESC LIMIT $3 OFFSET $4`, project, roleName, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Conversation
    for rows.Next() {
        var c Conversation
        var tagsJSON []byte
        if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Project, &tagsJSON, &c.Notes, &c.Created, &c.Updated); err != nil {
            return nil, err
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &c.Tags) }
        out = append(out, c)
    }
    return out, rows.Err()
}

// DeleteConversation deletes a conversation by id and returns affected rows.
func DeleteConversation(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM conversations WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
