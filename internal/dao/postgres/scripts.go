package postgres

import (
    "context"
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "encoding/json"
    "errors"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Script struct {
    ID              string
    Title           string
    Description     sql.NullString
    Motivation      sql.NullString
    Notes           sql.NullString
    ScriptContentID string // hex-encoded sha256 bytea
    RoleName        string
    Tags            map[string]any
    Created         sql.NullTime
    Updated         sql.NullTime
}

// InsertScriptContent ensures a content row exists for the given text and returns its hex id.
func InsertScriptContent(ctx context.Context, db *pgxpool.Pool, body string) (string, error) {
    canon := CanonicalizeText(body)
    if strings.TrimSpace(canon) == "" { return "", errors.New("empty script content") }
    sum := sha256.Sum256([]byte(canon))
    idHex := hex.EncodeToString(sum[:])
    // Insert if missing; use decode(hex,'hex') to convert to bytea
    _, err := db.Exec(ctx, `INSERT INTO scripts_content (id, script_content)
                            VALUES (decode($1,'hex'), $2)
                            ON CONFLICT (id) DO NOTHING`, idHex, canon)
    if err != nil { return "", err }
    return idHex, nil
}

// UpsertScript inserts a new script when ID is empty, otherwise updates by ID.
func UpsertScript(ctx context.Context, db *pgxpool.Pool, s *Script) error {
    var tagsJSON []byte
    if s.Tags != nil { tagsJSON, _ = json.Marshal(s.Tags) }
    if strings.TrimSpace(s.ID) == "" {
        q := `INSERT INTO scripts (title, description, motivation, notes, script_content_id, role_name, tags)
              VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), decode($5,'hex'), $6, COALESCE($7,'{}'::jsonb))
              RETURNING id::text, created, updated`
        return db.QueryRow(ctx, q, s.Title, stringOrEmpty(s.Description), stringOrEmpty(s.Motivation), stringOrEmpty(s.Notes), s.ScriptContentID, s.RoleName, tagsJSON).
            Scan(&s.ID, &s.Created, &s.Updated)
    }
    q := `UPDATE scripts
          SET title=$2,
              description=NULLIF($3,''),
              motivation=NULLIF($4,''),
              notes=NULLIF($5,''),
              script_content_id=decode($6,'hex'),
              role_name=$7,
              tags=COALESCE($8,'{}'::jsonb),
              updated=now()
          WHERE id=$1::uuid
          RETURNING created, updated`
    return db.QueryRow(ctx, q, s.ID, s.Title, stringOrEmpty(s.Description), stringOrEmpty(s.Motivation), stringOrEmpty(s.Notes), s.ScriptContentID, s.RoleName, tagsJSON).
        Scan(&s.Created, &s.Updated)
}

// GetScriptByID returns a script by id.
func GetScriptByID(ctx context.Context, db *pgxpool.Pool, id string) (*Script, error) {
    q := `SELECT id::text, title, description, motivation, notes, encode(script_content_id,'hex') AS cid, role_name, tags, created, updated
          FROM scripts WHERE id=$1::uuid`
    var s Script
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, id).Scan(&s.ID, &s.Title, &s.Description, &s.Motivation, &s.Notes, &s.ScriptContentID, &s.RoleName, &tagsJSON, &s.Created, &s.Updated); err != nil {
        return nil, err
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
    return &s, nil
}

// ListScripts lists scripts for a role, ordered by updated desc then created desc.
func ListScripts(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Script, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT id::text, title, description, motivation, notes, encode(script_content_id,'hex'), role_name, tags, created, updated
          FROM scripts WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Script
    for rows.Next() {
        var s Script
        var tagsJSON []byte
        if err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.Motivation, &s.Notes, &s.ScriptContentID, &s.RoleName, &tagsJSON, &s.Created, &s.Updated); err != nil {
            return nil, err
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
        out = append(out, s)
    }
    return out, rows.Err()
}

// DeleteScript removes a script by id.
func DeleteScript(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM scripts WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
