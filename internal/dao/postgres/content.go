package postgres

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

// New content API working with messages_content (id BIGSERIAL, text_content, json_content)

type ContentRecord struct {
    ID          string
    TextContent string
    JSONContent []byte // raw JSON or nil
}

// InsertContent inserts a new content row and returns its numeric id.
func InsertContent(ctx context.Context, db *pgxpool.Pool, text string, jsonPayload []byte) (string, error) {
    if strings.TrimSpace(text) == "" {
        return "", errors.New("empty content")
    }
    q := `INSERT INTO messages_content (text_content, json_content) VALUES ($1, $2) RETURNING id::text`
    var id string
    if err := db.QueryRow(ctx, q, text, jsonOrNil(jsonPayload)).Scan(&id); err != nil {
        return "", err
    }
    return id, nil
}

// GetContent fetches a content row by id.
func GetContent(ctx context.Context, db *pgxpool.Pool, id string) (ContentRecord, error) {
    var out ContentRecord
    if strings.TrimSpace(id) == "" { return out, errors.New("invalid id") }
    q := `SELECT id::text, text_content, COALESCE(json_content,'null'::jsonb) FROM messages_content WHERE id=$1::uuid`
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

// CanonicalizeText normalizes text for hashing/deduplication.
func CanonicalizeText(body string) string {
    s := strings.ReplaceAll(body, "\r\n", "\n")
    s = strings.ReplaceAll(s, "\r", "\n")
    s = strings.TrimSpace(s)
    lines := strings.Split(s, "\n")
    for i := range lines {
        lines[i] = strings.TrimRight(lines[i], " \t")
    }
    return strings.Join(lines, "\n")
}

// HashTextSHA256 returns a hex-encoded SHA-256 hash of the canonicalized text.
func HashTextSHA256(body string) string {
    canon := CanonicalizeText(body)
    sum := sha256.Sum256([]byte(canon))
    return hex.EncodeToString(sum[:])
}
