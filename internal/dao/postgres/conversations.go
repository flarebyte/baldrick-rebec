package postgres

import (
    "context"
    "database/sql"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Conversation struct {
    ID          int64
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Project     sql.NullString
    Tags        []string
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertConversation inserts a new conversation if ID==0, otherwise updates the existing one.
func UpsertConversation(ctx context.Context, db *pgxpool.Pool, c *Conversation) error {
    if c.ID > 0 {
        q := `UPDATE conversations
              SET title=$2, description=NULLIF($3,''), project=NULLIF($4,''), tags=$5::text[], notes=NULLIF($6,''), updated=now()
              WHERE id=$1 RETURNING created, updated`
        return db.QueryRow(ctx, q, c.ID, c.Title, stringOrEmpty(c.Description), stringOrEmpty(c.Project), c.Tags, stringOrEmpty(c.Notes)).Scan(&c.Created, &c.Updated)
    }
    q := `INSERT INTO conversations (title, description, project, tags, notes)
          VALUES ($1, NULLIF($2,''), NULLIF($3,''), $4::text[], NULLIF($5,''))
          RETURNING id, created, updated`
    return db.QueryRow(ctx, q, c.Title, stringOrEmpty(c.Description), stringOrEmpty(c.Project), c.Tags, stringOrEmpty(c.Notes)).Scan(&c.ID, &c.Created, &c.Updated)
}

// GetConversationByID returns a conversation by its id.
func GetConversationByID(ctx context.Context, db *pgxpool.Pool, id int64) (*Conversation, error) {
    q := `SELECT id, title, description, project, tags, notes, created, updated FROM conversations WHERE id=$1`
    var c Conversation
    var tags []string
    if err := db.QueryRow(ctx, q, id).Scan(&c.ID, &c.Title, &c.Description, &c.Project, &tags, &c.Notes, &c.Created, &c.Updated); err != nil {
        return nil, err
    }
    c.Tags = tags
    return &c, nil
}

// ListConversations lists conversations, optionally filtered by project, with pagination.
func ListConversations(ctx context.Context, db *pgxpool.Pool, project string, limit, offset int) ([]Conversation, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    if strings.TrimSpace(project) == "" {
        rows, err = db.Query(ctx, `SELECT id, title, description, project, tags, notes, created, updated FROM conversations ORDER BY created DESC LIMIT $1 OFFSET $2`, limit, offset)
    } else {
        rows, err = db.Query(ctx, `SELECT id, title, description, project, tags, notes, created, updated FROM conversations WHERE project=$1 ORDER BY created DESC LIMIT $2 OFFSET $3`, project, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Conversation
    for rows.Next() {
        var c Conversation
        var tags []string
        if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Project, &tags, &c.Notes, &c.Created, &c.Updated); err != nil {
            return nil, err
        }
        c.Tags = tags
        out = append(out, c)
    }
    return out, rows.Err()
}

// DeleteConversation deletes a conversation by id and returns affected rows.
func DeleteConversation(ctx context.Context, db *pgxpool.Pool, id int64) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM conversations WHERE id=$1`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
