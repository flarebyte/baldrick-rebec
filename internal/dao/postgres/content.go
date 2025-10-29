package postgres

import (
    "context"
    "errors"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

// New content API working with messages_content (id BIGSERIAL, text_content, json_content)

type ContentRecord struct {
    ID          int64
    TextContent string
    JSONContent []byte // raw JSON or nil
}

// InsertContent inserts a new content row and returns its numeric id.
func InsertContent(ctx context.Context, db *pgxpool.Pool, text string, jsonPayload []byte) (int64, error) {
    if strings.TrimSpace(text) == "" {
        return 0, errors.New("empty content")
    }
    q := `INSERT INTO messages_content (text_content, json_content) VALUES ($1, $2) RETURNING id`
    var id int64
    if err := db.QueryRow(ctx, q, text, jsonOrNil(jsonPayload)).Scan(&id); err != nil {
        return 0, err
    }
    return id, nil
}

// GetContent fetches a content row by id.
func GetContent(ctx context.Context, db *pgxpool.Pool, id int64) (ContentRecord, error) {
    var out ContentRecord
    if id <= 0 { return out, errors.New("invalid id") }
    q := `SELECT id, text_content, COALESCE(json_content,'null'::jsonb) FROM messages_content WHERE id=$1`
    var jsonb []byte
    if err := db.QueryRow(ctx, q, id).Scan(&out.ID, &out.TextContent, &jsonb); err != nil {
        return out, err
    }
    if string(jsonb) != "null" { out.JSONContent = jsonb }
    return out, nil
}

// Back-compat helpers expected by db init/scaffold paths
func EnsureContentSchema(ctx context.Context, db *pgxpool.Pool) error {
    _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS messages_content (
        id BIGSERIAL PRIMARY KEY,
        text_content TEXT NOT NULL,
        json_content JSONB,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`)
    return err
}

func EnsureFTSIndex(ctx context.Context, db *pgxpool.Pool) error {
    _, err := db.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_messages_content_fts
             ON messages_content USING GIN (to_tsvector('simple', text_content))`)
    return err
}
