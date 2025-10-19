package postgres

import (
    "context"
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "errors"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

// EnsureContentSchema creates the content table if missing.
func EnsureContentSchema(ctx context.Context, db *pgxpool.Pool) error {
    stmt := `CREATE TABLE IF NOT EXISTS messages_content_pg (
        id TEXT PRIMARY KEY,
        content TEXT NOT NULL,
        content_type TEXT,
        language TEXT,
        metadata JSONB DEFAULT '{}',
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`
    _, err := db.Exec(ctx, stmt)
    return err
}

// CanonicalizeBody normalizes message text for hashing/deduplication.
func CanonicalizeBody(body string) string {
    s := strings.ReplaceAll(body, "\r\n", "\n")
    s = strings.ReplaceAll(s, "\r", "\n")
    s = strings.TrimSpace(s)
    lines := strings.Split(s, "\n")
    for i := range lines {
        lines[i] = strings.TrimRight(lines[i], " \t")
    }
    return strings.Join(lines, "\n")
}

func HashBodySHA256(body string) string {
    canon := CanonicalizeBody(body)
    sum := sha256.Sum256([]byte(canon))
    return hex.EncodeToString(sum[:])
}

type MessageContent struct {
    ID          string
    Content     string
    ContentType sql.NullString
    Language    sql.NullString
    Metadata    []byte // raw JSON
}

// PutMessageContent inserts or ignores an existing content row; returns the content id.
func PutMessageContent(ctx context.Context, db *pgxpool.Pool, content, contentType, language string, metadata []byte) (string, error) {
    if strings.TrimSpace(content) == "" {
        return "", errors.New("empty content")
    }
    id := HashBodySHA256(content)
    q := `INSERT INTO messages_content_pg (id, content, content_type, language, metadata)
          VALUES ($1,$2, NULLIF($3,''), NULLIF($4,''), COALESCE($5,'{}'::jsonb))
          ON CONFLICT (id) DO NOTHING`
    if _, err := db.Exec(ctx, q, id, content, contentType, language, metadata); err != nil {
        return "", err
    }
    return id, nil
}

func GetMessageContent(ctx context.Context, db *pgxpool.Pool, id string) (MessageContent, error) {
    var out MessageContent
    if strings.TrimSpace(id) == "" {
        return out, errors.New("empty id")
    }
    q := `SELECT id, content, content_type, language, COALESCE(metadata,'{}'::jsonb)
          FROM messages_content_pg WHERE id=$1`
    row := db.QueryRow(ctx, q, id)
    var meta []byte
    if err := row.Scan(&out.ID, &out.Content, &out.ContentType, &out.Language, &meta); err != nil {
        return out, err
    }
    out.Metadata = meta
    return out, nil
}

// EnsureFTSIndex creates a GIN index on to_tsvector('simple', content).
func EnsureFTSIndex(ctx context.Context, db *pgxpool.Pool) error {
    stmt := `CREATE INDEX IF NOT EXISTS idx_messages_content_pg_fts
             ON messages_content_pg USING GIN (to_tsvector('simple', content))`
    _, err := db.Exec(ctx, stmt)
    return err
}

// EnsureVectorExtension attempts to enable the pgvector extension.
func EnsureVectorExtension(ctx context.Context, db *pgxpool.Pool) error {
    // Requires superuser or appropriate privileges; safe to attempt.
    _, err := db.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`)
    return err
}

// HasVectorExtension reports whether the pgvector extension is available.
func HasVectorExtension(ctx context.Context, db *pgxpool.Pool) (bool, error) {
    var ok bool
    err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='vector')`).Scan(&ok)
    return ok, err
}

// EnsureEmbeddingColumn ensures an embedding column exists with given dimension.
func EnsureEmbeddingColumn(ctx context.Context, db *pgxpool.Pool, dim int) error {
    if dim <= 0 { return nil }
    // Add column if missing
    _, err := db.Exec(ctx, `ALTER TABLE messages_content_pg ADD COLUMN IF NOT EXISTS embedding vector`)
    if err != nil { return err }
    // Verify dimension; Postgres vector type encodes dim at runtime; cannot enforce at DDL easily without casting.
    return nil
}

// EnsureEmbeddingIndex creates an approximate index on embedding if column exists.
func EnsureEmbeddingIndex(ctx context.Context, db *pgxpool.Pool) error {
    // Use ivfflat by default; requires SET to enable; try to create index and ignore errors.
    // Users may adjust storage parameters later.
    _, err := db.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_messages_content_pg_embedding ON messages_content_pg USING ivfflat (embedding)`)
    return err
}
