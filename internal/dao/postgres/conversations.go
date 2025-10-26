package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Conversation struct {
    ID          string
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Project     sql.NullString
    Tags        []string
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertConversation inserts or updates a conversation by id.
func UpsertConversation(ctx context.Context, db *pgxpool.Pool, c *Conversation) error {
    q := `INSERT INTO conversations (id, title, description, project, tags, notes)
          VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), $5::text[], NULLIF($6,''))
          ON CONFLICT (id) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            project = EXCLUDED.project,
            tags = EXCLUDED.tags,
            notes = EXCLUDED.notes,
            updated = now()
          RETURNING created, updated`
    return db.QueryRow(ctx, q, c.ID, c.Title, stringOrEmpty(c.Description), stringOrEmpty(c.Project), c.Tags, stringOrEmpty(c.Notes)).Scan(&c.Created, &c.Updated)
}

