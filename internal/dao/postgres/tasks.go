package postgres

import (
    "context"
    "database/sql"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Task struct {
    ID         int64
    WorkflowID string
    Command    string
    Variant    string
    Title      sql.NullString
    Description sql.NullString
    Motivation sql.NullString
    Version    string
    Created    sql.NullTime
    Notes      sql.NullString
    Shell      sql.NullString
    Run        sql.NullString
    Timeout    sql.NullString // textual interval
    Tags       []string
    Level      sql.NullString // h1..h6
}

// UpsertTask inserts or updates a task identified by (workflow_id, name, version).
func UpsertTask(ctx context.Context, db *pgxpool.Pool, t *Task) error {
    // Normalize variant: accept either full selector (command/...)
    // or suffix to be prefixed with command; if empty, use command
    v := strings.TrimSpace(t.Variant)
    c := strings.TrimSpace(t.Command)
    switch {
    case v == "" && c != "":
        t.Variant = c
    case v != "" && c != "":
        if strings.Contains(v, "/") {
            // trust provided selector; also ensure command matches prefix
            // if not matching, still store as-is and set command from prefix
            parts := strings.SplitN(v, "/", 2)
            t.Command = parts[0]
            t.Variant = v
        } else {
            t.Variant = c + "/" + v
        }
    case v != "" && c == "":
        t.Variant = v
        if strings.Contains(v, "/") {
            t.Command = strings.SplitN(v, "/", 2)[0]
        } else {
            t.Command = v
        }
    default:
        // both empty; invalid later by DB constraints if referenced
    }
    q := `INSERT INTO tasks (
            workflow_id, command, variant, title, description, motivation, version,
            notes, shell, run, timeout, tags, level
          ) VALUES (
            $1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), $7,
            NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), CASE WHEN $11='' THEN NULL ELSE $11::interval END, $12::text[], NULLIF($13,'')
          )
          ON CONFLICT (workflow_id, variant, version) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            motivation = EXCLUDED.motivation,
            notes = EXCLUDED.notes,
            shell = EXCLUDED.shell,
            run = EXCLUDED.run,
            timeout = EXCLUDED.timeout,
            tags = EXCLUDED.tags,
            level = EXCLUDED.level
          RETURNING id, created`
    var id int64
    var created sql.NullTime
    if err := db.QueryRow(ctx, q,
        t.WorkflowID, t.Command, t.Variant, stringOrEmpty(t.Title), stringOrEmpty(t.Description), stringOrEmpty(t.Motivation), t.Version,
        stringOrEmpty(t.Notes), stringOrEmpty(t.Shell), stringOrEmpty(t.Run), stringOrEmpty(t.Timeout), t.Tags, stringOrEmpty(t.Level),
    ).Scan(&id, &created); err != nil {
        return err
    }
    t.ID = id
    t.Created = created
    return nil
}

// GetTaskByID fetches a task by numeric id.
func GetTaskByID(ctx context.Context, db *pgxpool.Pool, id int64) (*Task, error) {
    q := `SELECT id, workflow_id, command, variant, title, description, motivation, version,
                 notes, shell, run, timeout::text, tags, level, created
          FROM tasks WHERE id=$1`
    var t Task
    var tags []string
    if err := db.QueryRow(ctx, q, id).Scan(
        &t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation, &t.Version,
        &t.Notes, &t.Shell, &t.Run, &t.Timeout, &tags, &t.Level, &t.Created,
    ); err != nil {
        return nil, err
    }
    t.Tags = tags
    return &t, nil
}

// GetTaskByKey fetches a task by (workflow_id, name, version).
func GetTaskByKey(ctx context.Context, db *pgxpool.Pool, variant, ver string) (*Task, error) {
    q := `SELECT id, workflow_id, command, variant, title, description, motivation, version,
                 notes, shell, run, timeout::text, tags, level, created
          FROM tasks WHERE variant=$1 AND version=$2`
    var t Task
    var tags []string
    if err := db.QueryRow(ctx, q, variant, ver).Scan(
        &t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation, &t.Version,
        &t.Notes, &t.Shell, &t.Run, &t.Timeout, &tags, &t.Level, &t.Created,
    ); err != nil {
        return nil, err
    }
    t.Tags = tags
    return &t, nil
}

// ListTasks lists tasks with optional workflow filter.
func ListTasks(ctx context.Context, db *pgxpool.Pool, workflow string, limit, offset int) ([]Task, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    if stringsTrim(workflow) == "" {
        rows, err = db.Query(ctx, `SELECT id, workflow_id, command, variant, title, description, motivation, version,
                                        notes, shell, run, timeout::text, tags, level, created
                                   FROM tasks ORDER BY workflow_id, command, variant, version LIMIT $1 OFFSET $2`, limit, offset)
    } else {
        rows, err = db.Query(ctx, `SELECT id, workflow_id, command, variant, title, description, motivation, version,
                                        notes, shell, run, timeout::text, tags, level, created
                                   FROM tasks WHERE workflow_id=$1 ORDER BY command, variant, version LIMIT $2 OFFSET $3`, workflow, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Task
    for rows.Next() {
        var t Task
        var tags []string
        if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation, &t.Version,
            &t.Notes, &t.Shell, &t.Run, &t.Timeout, &tags, &t.Level, &t.Created); err != nil {
            return nil, err
        }
        t.Tags = tags
        out = append(out, t)
    }
    return out, rows.Err()
}

// DeleteTaskByID deletes a task by id.
func DeleteTaskByID(ctx context.Context, db *pgxpool.Pool, id int64) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM tasks WHERE id=$1`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

// DeleteTaskByKey deletes a task by (workflow_id, name, version).
func DeleteTaskByKey(ctx context.Context, db *pgxpool.Pool, wf, variant, ver string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM tasks WHERE workflow_id=$1 AND variant=$2 AND version=$3`, wf, variant, ver)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

// helpers
type pgxRows interface{ Next() bool; Scan(...any) error; Close(); Err() error }
func stringsTrim(s string) string { return strings.TrimSpace(s) }
