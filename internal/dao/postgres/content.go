package postgres

import (
    "context"
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "errors"
    "strings"
)

// EnsureContentSchema creates the content table if missing.
func EnsureContentSchema(ctx context.Context, db *sql.DB) error {
    stmt := `CREATE TABLE IF NOT EXISTS messages_content_pg (
        id TEXT PRIMARY KEY,
        content TEXT NOT NULL,
        content_type TEXT,
        language TEXT,
        metadata JSONB DEFAULT '{}',
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`
    _, err := db.ExecContext(ctx, stmt)
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
func PutMessageContent(ctx context.Context, db *sql.DB, content, contentType, language string, metadata []byte) (string, error) {
    if strings.TrimSpace(content) == "" {
        return "", errors.New("empty content")
    }
    id := HashBodySHA256(content)
    q := `INSERT INTO messages_content_pg (id, content, content_type, language, metadata)
          VALUES ($1,$2, NULLIF($3,''), NULLIF($4,''), COALESCE($5,'{}'::jsonb))
          ON CONFLICT (id) DO NOTHING`
    if _, err := db.ExecContext(ctx, q, id, content, contentType, language, metadata); err != nil {
        return "", err
    }
    return id, nil
}

func GetMessageContent(ctx context.Context, db *sql.DB, id string) (MessageContent, error) {
    var out MessageContent
    if strings.TrimSpace(id) == "" {
        return out, errors.New("empty id")
    }
    q := `SELECT id, content, content_type, language, COALESCE(metadata,'{}'::jsonb)
          FROM messages_content_pg WHERE id=$1`
    row := db.QueryRowContext(ctx, q, id)
    var meta []byte
    if err := row.Scan(&out.ID, &out.Content, &out.ContentType, &out.Language, &meta); err != nil {
        return out, err
    }
    out.Metadata = meta
    return out, nil
}

