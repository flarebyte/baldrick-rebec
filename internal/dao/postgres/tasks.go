package postgres

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Task struct {
    ID         string
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
    RunScriptID sql.NullString
    Timeout    sql.NullString // textual interval
    ToolWorkspaceID sql.NullString
    Tags       map[string]any
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
    // Ensure registry binding for the selector to its owning workflow
    if strings.TrimSpace(t.WorkflowID) != "" && strings.TrimSpace(t.Variant) != "" {
        if _, err := db.Exec(ctx, `INSERT INTO task_variants (variant, workflow_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, t.Variant, t.WorkflowID); err != nil {
            return err
        }
        var owner string
        if err := db.QueryRow(ctx, `SELECT workflow_id FROM task_variants WHERE variant=$1`, t.Variant).Scan(&owner); err != nil {
            return err
        }
        if owner != t.WorkflowID {
            return fmt.Errorf("variant %q already owned by workflow %q (requested %q)", t.Variant, owner, t.WorkflowID)
        }
    }
    q := `INSERT INTO tasks (
            command, variant, title, description, motivation, version,
            notes, shell, run_script_id, timeout, tool_workspace_id, tags, level
          ) VALUES (
            $1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6,
            NULLIF($7,''), NULLIF($8,''), CASE WHEN $9='' THEN NULL ELSE $9::uuid END, CASE WHEN $10='' THEN NULL ELSE $10::interval END,
            CASE WHEN $11='' THEN NULL ELSE $11::uuid END, COALESCE($12,'{}'::jsonb), NULLIF($13,'')
          )
          ON CONFLICT (variant, version) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            motivation = EXCLUDED.motivation,
            notes = EXCLUDED.notes,
            shell = EXCLUDED.shell,
            run_script_id = EXCLUDED.run_script_id,
            timeout = EXCLUDED.timeout,
            tool_workspace_id = EXCLUDED.tool_workspace_id,
            tags = EXCLUDED.tags,
            level = EXCLUDED.level
          RETURNING id, created`
    var id string
    var created sql.NullTime
    var tagsJSON []byte
    if t.Tags != nil { tagsJSON, _ = json.Marshal(t.Tags) }
    if err := db.QueryRow(ctx, q,
        t.Command, t.Variant, stringOrEmpty(t.Title), stringOrEmpty(t.Description), stringOrEmpty(t.Motivation), t.Version,
        stringOrEmpty(t.Notes), stringOrEmpty(t.Shell), stringOrEmpty(t.RunScriptID), stringOrEmpty(t.Timeout), stringOrEmpty(t.ToolWorkspaceID), tagsJSON, stringOrEmpty(t.Level),
    ).Scan(&id, &created); err != nil {
        return err
    }
    t.ID = id
    t.Created = created
    return nil
}

// GetTaskByID fetches a task by numeric id.
func GetTaskByID(ctx context.Context, db *pgxpool.Pool, id string) (*Task, error) {
    q := `SELECT t.id::text, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation, t.version,
                 t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
          FROM tasks t
          LEFT JOIN task_variants tv ON tv.variant = t.variant
          WHERE t.id=$1::uuid`
    var t Task
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, id).Scan(
        &t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation, &t.Version,
        &t.Notes, &t.Shell, &t.RunScriptID, &t.Timeout, &t.ToolWorkspaceID, &tagsJSON, &t.Level, &t.Created,
    ); err != nil {
        return nil, err
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
    return &t, nil
}

// GetTaskByKey fetches a task by (workflow_id, name, version).
func GetTaskByKey(ctx context.Context, db *pgxpool.Pool, variant, ver string) (*Task, error) {
    q := `SELECT t.id, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation, t.version,
                 t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
          FROM tasks t
          LEFT JOIN task_variants tv ON tv.variant = t.variant
          WHERE t.variant=$1 AND t.version=$2`
    var t Task
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, variant, ver).Scan(
        &t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation, &t.Version,
        &t.Notes, &t.Shell, &t.RunScriptID, &t.Timeout, &t.ToolWorkspaceID, &tagsJSON, &t.Level, &t.Created,
    ); err != nil {
        return nil, err
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
    return &t, nil
}

// ListTasks lists tasks with optional workflow filter.
func ListTasks(ctx context.Context, db *pgxpool.Pool, workflow, roleName string, limit, offset int) ([]Task, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    if stringsTrim(workflow) == "" {
        rows, err = db.Query(ctx, `SELECT t.id, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation, t.version,
                                        t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
                                   FROM tasks t
                                   LEFT JOIN task_variants tv ON tv.variant = t.variant
                                   WHERE t.role_name=$1
                                   ORDER BY t.variant ASC, t.version ASC LIMIT $2 OFFSET $3`, roleName, limit, offset)
    } else {
        rows, err = db.Query(ctx, `SELECT t.id, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation, t.version,
                                        t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
                                   FROM tasks t
                                   LEFT JOIN task_variants tv ON tv.variant = t.variant
                                   WHERE tv.workflow_id=$1 AND t.role_name=$2
                                   ORDER BY t.variant ASC, t.version ASC LIMIT $3 OFFSET $4`, workflow, roleName, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Task
    for rows.Next() {
        var t Task
        var tagsJSON []byte
        if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation, &t.Version,
            &t.Notes, &t.Shell, &t.RunScriptID, &t.Timeout, &t.ToolWorkspaceID, &tagsJSON, &t.Level, &t.Created); err != nil {
            return nil, err
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
        out = append(out, t)
    }
    return out, rows.Err()
}

// DeleteTaskByID deletes a task by id.
func DeleteTaskByID(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM tasks WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

// DeleteTaskByKey deletes a task by (workflow_id, name, version).
func DeleteTaskByKey(ctx context.Context, db *pgxpool.Pool, variant, ver string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM tasks WHERE variant=$1 AND version=$2`, variant, ver)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

// helpers
type pgxRows interface{ Next() bool; Scan(...any) error; Close(); Err() error }
func stringsTrim(s string) string { return strings.TrimSpace(s) }
