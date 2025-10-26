package postgres

import (
    "context"
    "database/sql"

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
