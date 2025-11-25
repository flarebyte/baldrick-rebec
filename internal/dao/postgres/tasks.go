package postgres

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
)

type Task struct {
    ID         string
    WorkflowID string
    Command    string
    Variant    string
    RoleName   string
    Title      sql.NullString
    Description sql.NullString
    Motivation sql.NullString
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
    // Prepare a privacy-safe summary for error context
    summarize := func(tk *Task) string { return taskSummary(tk) }

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
            return fmt.Errorf("upsert task: ensure variant owner failed: %w; %s", err, summarize(t))
        }
        var owner string
        if err := db.QueryRow(ctx, `SELECT workflow_id FROM task_variants WHERE variant=$1`, t.Variant).Scan(&owner); err != nil {
            return fmt.Errorf("upsert task: check variant owner failed: %w; %s", err, summarize(t))
        }
        if owner != t.WorkflowID {
            return fmt.Errorf("variant %q already owned by workflow %q (requested %q)", t.Variant, owner, t.WorkflowID)
        }
    }
    q := `INSERT INTO tasks (
            command, variant, role_name, title, description, motivation,
            notes, shell, run_script_id, timeout, tool_workspace_id, tags, level
          ) VALUES (
            $1, $2, COALESCE(NULLIF($3,''),'user'), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''),
            NULLIF($7,''), NULLIF($8,''), CASE WHEN $9='' THEN NULL ELSE $9::uuid END, CASE WHEN $10='' THEN NULL ELSE $10::interval END,
            CASE WHEN $11='' THEN NULL ELSE $11::uuid END, COALESCE($12,'{}'::jsonb), NULLIF($13,'')
          )
          ON CONFLICT (variant) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            motivation = EXCLUDED.motivation,
            notes = EXCLUDED.notes,
            shell = EXCLUDED.shell,
            run_script_id = EXCLUDED.run_script_id,
            timeout = EXCLUDED.timeout,
            tool_workspace_id = EXCLUDED.tool_workspace_id,
            tags = EXCLUDED.tags,
            level = EXCLUDED.level,
            role_name = EXCLUDED.role_name
          RETURNING id, created`
    var id string
    var created sql.NullTime
    var tagsJSON []byte
    if t.Tags != nil { tagsJSON, _ = json.Marshal(t.Tags) }
    if err := db.QueryRow(ctx, q,
        t.Command, t.Variant, t.RoleName, stringOrEmpty(t.Title), stringOrEmpty(t.Description), stringOrEmpty(t.Motivation),
        stringOrEmpty(t.Notes), stringOrEmpty(t.Shell), stringOrEmpty(t.RunScriptID), stringOrEmpty(t.Timeout), stringOrEmpty(t.ToolWorkspaceID), tagsJSON, stringOrEmpty(t.Level),
    ).Scan(&id, &created); err != nil {
        return fmt.Errorf("upsert task: write failed: %w; %s", err, summarize(t))
    }
    t.ID = id
    t.Created = created
    // Ensure a Task vertex exists/updated in AGE graph
    // Best-effort: AGE graph writes are optional in this deployment. Avoid failing the upsert
    // if the graph is unavailable or lacks privileges; diagnostics are available via age-status.
    _ = EnsureTaskVertex(ctx, db, t.ID, t.Variant, t.Command)
    return nil
}

// GetTaskByID fetches a task by numeric id.
func GetTaskByID(ctx context.Context, db *pgxpool.Pool, id string) (*Task, error) {
    q := `SELECT t.id::text, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation,
                 t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
          FROM tasks t
          LEFT JOIN task_variants tv ON tv.variant = t.variant
          WHERE t.id=$1::uuid`
    var t Task
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, id).Scan(
        &t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation,
        &t.Notes, &t.Shell, &t.RunScriptID, &t.Timeout, &t.ToolWorkspaceID, &tagsJSON, &t.Level, &t.Created,
    ); err != nil {
        return nil, dbutil.ErrWrap("task.get", err, dbutil.ParamSummary("id", id))
    }
    if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
    return &t, nil
}

// GetTaskByVariant fetches a task by variant.
func GetTaskByVariant(ctx context.Context, db *pgxpool.Pool, variant string) (*Task, error) {
    q := `SELECT t.id, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation,
                 t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
          FROM tasks t
          LEFT JOIN task_variants tv ON tv.variant = t.variant
          WHERE t.variant=$1`
    var t Task
    var tagsJSON []byte
    if err := db.QueryRow(ctx, q, variant).Scan(
        &t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation,
        &t.Notes, &t.Shell, &t.RunScriptID, &t.Timeout, &t.ToolWorkspaceID, &tagsJSON, &t.Level, &t.Created,
    ); err != nil {
        return nil, dbutil.ErrWrap("task.get", err, dbutil.ParamSummary("variant", variant))
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
        rows, err = db.Query(ctx, `SELECT t.id, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation,
                                        t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
                                   FROM tasks t
                                   LEFT JOIN task_variants tv ON tv.variant = t.variant
                                   WHERE t.role_name=$1
                                   ORDER BY t.variant ASC LIMIT $2 OFFSET $3`, roleName, limit, offset)
    } else {
        rows, err = db.Query(ctx, `SELECT t.id, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation,
                                        t.notes, t.shell, t.run_script_id::text, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.created
                                   FROM tasks t
                                   LEFT JOIN task_variants tv ON tv.variant = t.variant
                                   WHERE tv.workflow_id=$1 AND t.role_name=$2
                                   ORDER BY t.variant ASC LIMIT $3 OFFSET $4`, workflow, roleName, limit, offset)
    }
    if err != nil { return nil, dbutil.ErrWrap("task.list", err, dbutil.ParamSummary("workflow", workflow), dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset)) }
    defer rows.Close()
    var out []Task
    for rows.Next() {
        var t Task
        var tagsJSON []byte
        if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation,
            &t.Notes, &t.Shell, &t.RunScriptID, &t.Timeout, &t.ToolWorkspaceID, &tagsJSON, &t.Level, &t.Created); err != nil {
            return nil, dbutil.ErrWrap("task.list.scan", err)
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
        out = append(out, t)
    }
    if err := rows.Err(); err != nil { return nil, dbutil.ErrWrap("task.list", err) }
    return out, nil
}

// DeleteTaskByID deletes a task by id.
func DeleteTaskByID(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM tasks WHERE id=$1::uuid`, id)
    if err != nil { return 0, dbutil.ErrWrap("task.delete", err, dbutil.ParamSummary("id", id)) }
    return ct.RowsAffected(), nil
}

// DeleteTaskByKey deletes a task by (workflow_id, name, version).
func DeleteTaskByKey(ctx context.Context, db *pgxpool.Pool, variant, _ string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM tasks WHERE variant=$1`, variant)
    if err != nil { return 0, dbutil.ErrWrap("task.delete", err, dbutil.ParamSummary("variant", variant)) }
    return ct.RowsAffected(), nil
}

// helpers
type pgxRows interface{ Next() bool; Scan(...any) error; Close(); Err() error }
func stringsTrim(s string) string { return strings.TrimSpace(s) }

// taskSummary builds a privacy-conscious summary of a Task for error context.
func taskSummary(t *Task) string {
    if t == nil { return "task=null" }
    parts := []string{
        dbutil.ParamSummary("command", t.Command),
        dbutil.ParamSummary("variant", t.Variant),
        dbutil.ParamSummary("workflow_id", t.WorkflowID),
        dbutil.ParamSummary("title", t.Title),
        dbutil.ParamSummary("description", t.Description),
        dbutil.ParamSummary("motivation", t.Motivation),
        dbutil.ParamSummary("notes", t.Notes),
        dbutil.ParamSummary("shell", t.Shell),
        dbutil.ParamSummary("run_script_id", t.RunScriptID),
        dbutil.ParamSummary("timeout", t.Timeout),
        dbutil.ParamSummary("tool_workspace_id", t.ToolWorkspaceID),
        dbutil.ParamSummary("level", t.Level),
    }
    // Tags: show size only to avoid leaking keys/values
    if t.Tags == nil { parts = append(parts, "tags=null") } else { parts = append(parts, fmt.Sprintf("tags=len=%d", len(t.Tags))) }
    return "task{" + strings.Join(parts, ",") + "}"
}
