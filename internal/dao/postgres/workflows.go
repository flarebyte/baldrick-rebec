package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Workflow struct {
    Name        string
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertWorkflow inserts or updates a workflow by name.
func UpsertWorkflow(ctx context.Context, db *pgxpool.Pool, w *Workflow) error {
    q := `INSERT INTO workflows (name, title, description, notes)
          VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))
          ON CONFLICT (name) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            notes = EXCLUDED.notes,
            updated = now()
          RETURNING created, updated`
    return db.QueryRow(ctx, q, w.Name, w.Title, stringOrEmpty(w.Description), stringOrEmpty(w.Notes)).Scan(&w.Created, &w.Updated)
}

