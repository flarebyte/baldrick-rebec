package postgres

import (
    "context"
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
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
    ComplexName     ScriptComplexName
    Archived        bool
    Created         sql.NullTime
    Updated         sql.NullTime
}

type ScriptComplexName struct {
    Name    string `json:"name"`
    Variant string `json:"variant"`
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
    if err != nil { return "", dbutil.ErrWrap("script_content.insert", err, dbutil.ParamSummary("id_hex", idHex), dbutil.ParamSummary("len", len(canon))) }
    return idHex, nil
}

// UpsertScript inserts a new script when ID is empty, otherwise updates by ID.
func UpsertScript(ctx context.Context, db *pgxpool.Pool, s *Script) error {
    var tagsJSON []byte
    if s.Tags != nil { tagsJSON, _ = json.Marshal(s.Tags) }
    // Ensure complex_name present: default name from Title if not provided; variant may be empty
    if strings.TrimSpace(s.ComplexName.Name) == "" {
        s.ComplexName.Name = strings.TrimSpace(s.Title)
    }
    cnJSON, _ := json.Marshal(map[string]string{"name": s.ComplexName.Name, "variant": s.ComplexName.Variant})
    if strings.TrimSpace(s.ID) == "" {
        q := `INSERT INTO scripts (title, description, motivation, notes, script_content_id, role_name, tags, complex_name, archived)
              VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), decode($5,'hex'), $6, COALESCE($7,'{}'::jsonb), COALESCE($8,'{"name":"","variant":""}'::jsonb), COALESCE($9,false))
              RETURNING id::text, created, updated`
        if err := db.QueryRow(ctx, q, s.Title, stringOrEmpty(s.Description), stringOrEmpty(s.Motivation), stringOrEmpty(s.Notes), s.ScriptContentID, s.RoleName, tagsJSON, string(cnJSON), s.Archived).
            Scan(&s.ID, &s.Created, &s.Updated); err != nil {
            return dbutil.ErrWrap("script.upsert.insert", err, dbutil.ParamSummary("title", s.Title), dbutil.ParamSummary("role", s.RoleName))
        }
        return nil
    }
    q := `UPDATE scripts
          SET title=$2,
              description=NULLIF($3,''),
              motivation=NULLIF($4,''),
              notes=NULLIF($5,''),
              script_content_id=decode($6,'hex'),
              role_name=$7,
              tags=COALESCE($8,'{}'::jsonb),
              complex_name=COALESCE($9,'{"name":"","variant":""}'::jsonb),
              archived=$10,
              updated=now()
          WHERE id=$1::uuid
          RETURNING created, updated`
    if err := db.QueryRow(ctx, q, s.ID, s.Title, stringOrEmpty(s.Description), stringOrEmpty(s.Motivation), stringOrEmpty(s.Notes), s.ScriptContentID, s.RoleName, tagsJSON, string(cnJSON), s.Archived).
        Scan(&s.Created, &s.Updated); err != nil {
        return dbutil.ErrWrap("script.upsert.update", err, dbutil.ParamSummary("id", s.ID), dbutil.ParamSummary("title", s.Title))
    }
    return nil
}

// GetScriptByID returns a script by id.
func GetScriptByID(ctx context.Context, db *pgxpool.Pool, id string) (*Script, error) {
    q := `SELECT id::text, title, description, motivation, notes, encode(script_content_id,'hex') AS cid, role_name, tags, complex_name, archived, created, updated
          FROM scripts WHERE id=$1::uuid`
    var s Script
    var tagsJSON []byte
    var cnJSON []byte
    if err := db.QueryRow(ctx, q, id).Scan(&s.ID, &s.Title, &s.Description, &s.Motivation, &s.Notes, &s.ScriptContentID, &s.RoleName, &tagsJSON, &cnJSON, &s.Archived, &s.Created, &s.Updated); err != nil {
        return nil, dbutil.ErrWrap("script.get", err, dbutil.ParamSummary("id", id))
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
    if len(cnJSON) > 0 {
        _ = json.Unmarshal(cnJSON, &s.ComplexName)
    }
    return &s, nil
}

// ListScripts lists scripts for a role, ordered by updated desc then created desc.
func ListScripts(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Script, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT id::text, title, description, motivation, notes, encode(script_content_id,'hex'), role_name, tags, complex_name, archived, created, updated
          FROM scripts WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, dbutil.ErrWrap("script.list", err, dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset)) }
    defer rows.Close()
    var out []Script
    for rows.Next() {
        var s Script
        var tagsJSON []byte
        var cnJSON []byte
        if err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.Motivation, &s.Notes, &s.ScriptContentID, &s.RoleName, &tagsJSON, &cnJSON, &s.Archived, &s.Created, &s.Updated); err != nil {
            return nil, dbutil.ErrWrap("script.list.scan", err, dbutil.ParamSummary("role", roleName))
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
        if len(cnJSON) > 0 { _ = json.Unmarshal(cnJSON, &s.ComplexName) }
        out = append(out, s)
    }
    if err := rows.Err(); err != nil { return nil, dbutil.ErrWrap("script.list", err, dbutil.ParamSummary("role", roleName)) }
    return out, nil
}

// GetScriptByComplexName performs exact lookup on (complex_name.name, complex_name.variant) with archived flag, ordering by recency.
func GetScriptByComplexName(ctx context.Context, db *pgxpool.Pool, name, variant string, archived bool) (*Script, error) {
    const q = `
        SELECT id::text, title, description, motivation, notes,
               encode(script_content_id,'hex') AS cid, role_name, tags, complex_name, archived, created, updated
        FROM scripts
        WHERE (complex_name->>'name') = $1
          AND (complex_name->>'variant') = $2
          AND archived = $3
        ORDER BY updated DESC
        LIMIT 1`
    var s Script
    var tagsJSON []byte
    var cnJSON []byte
    if err := db.QueryRow(ctx, q, name, variant, archived).
        Scan(&s.ID, &s.Title, &s.Description, &s.Motivation, &s.Notes, &s.ScriptContentID, &s.RoleName, &tagsJSON, &cnJSON, &s.Archived, &s.Created, &s.Updated); err != nil {
        return nil, dbutil.ErrWrap("script.get_by_complex_name", err, dbutil.ParamSummary("name", name), dbutil.ParamSummary("variant", variant), dbutil.ParamSummary("archived", archived))
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
    if len(cnJSON) > 0 { _ = json.Unmarshal(cnJSON, &s.ComplexName) }
    return &s, nil
}

// GetScriptByComplexNameRole performs exact lookup and constrains to a role when provided.
func GetScriptByComplexNameRole(ctx context.Context, db *pgxpool.Pool, name, variant string, archived bool, role string) (*Script, error) {
    const q = `
        SELECT id::text, title, description, motivation, notes,
               encode(script_content_id,'hex') AS cid, role_name, tags, complex_name, archived, created, updated
        FROM scripts
        WHERE (complex_name->>'name') = $1
          AND (complex_name->>'variant') = $2
          AND archived = $3
          AND role_name = $4
        ORDER BY updated DESC
        LIMIT 1`
    var s Script
    var tagsJSON []byte
    var cnJSON []byte
    if err := db.QueryRow(ctx, q, name, variant, archived, role).
        Scan(&s.ID, &s.Title, &s.Description, &s.Motivation, &s.Notes, &s.ScriptContentID, &s.RoleName, &tagsJSON, &cnJSON, &s.Archived, &s.Created, &s.Updated); err != nil {
        return nil, dbutil.ErrWrap("script.get_by_complex_name_role", err, dbutil.ParamSummary("name", name), dbutil.ParamSummary("variant", variant), dbutil.ParamSummary("archived", archived), dbutil.ParamSummary("role", role))
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &s.Tags) }
    if len(cnJSON) > 0 { _ = json.Unmarshal(cnJSON, &s.ComplexName) }
    return &s, nil
}

// DeleteScript removes a script by id.
func DeleteScript(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM scripts WHERE id=$1::uuid`, id)
    if err != nil { return 0, dbutil.ErrWrap("script.delete", err, dbutil.ParamSummary("id", id)) }
    return ct.RowsAffected(), nil
}

// GetScriptContent returns the script text by hex content id.
func GetScriptContent(ctx context.Context, db *pgxpool.Pool, contentHex string) (string, error) {
    if strings.TrimSpace(contentHex) == "" { return "", errors.New("empty content id") }
    var body string
    if err := db.QueryRow(ctx, `SELECT script_content FROM scripts_content WHERE id=decode($1,'hex')`, contentHex).Scan(&body); err != nil {
        return "", dbutil.ErrWrap("script_content.get", err, dbutil.ParamSummary("id_hex", contentHex))
    }
    return body, nil
}
