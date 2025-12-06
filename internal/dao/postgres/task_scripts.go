package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskScript represents an attachment row linking a task to a script under a logical name/alias.
type TaskScript struct {
	ID        string
	TaskID    string
	ScriptID  string
	Name      string
	Alias     sql.NullString
	CreatedAt sql.NullTime
}

const (
	pgErrUnique     = "23505"
	pgErrForeignKey = "23503"
)

// AddTaskScript attaches an existing script to a task under a logical name and optional alias.
func AddTaskScript(ctx context.Context, db *pgxpool.Pool, taskID, scriptID, name string, alias sql.NullString) (*TaskScript, error) {
	q := `INSERT INTO task_scripts (task_id, script_id, name, alias)
          VALUES ($1::uuid, $2::uuid, $3, NULLIF($4,''))
          RETURNING id::text, task_id::text, script_id::text, name, alias, created_at`
	var ts TaskScript
	var aliasArg string
	if alias.Valid {
		aliasArg = alias.String
	} else {
		aliasArg = ""
	}
	if err := db.QueryRow(ctx, q, taskID, scriptID, name, aliasArg).
		Scan(&ts.ID, &ts.TaskID, &ts.ScriptID, &ts.Name, &ts.Alias, &ts.CreatedAt); err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) {
			if pgerr.Code == pgErrUnique {
				return nil, dbutil.ErrWrap("task_script.add.conflict", err, dbutil.ParamSummary("task_id", taskID), dbutil.ParamSummary("name/alias", name))
			}
			if pgerr.Code == pgErrForeignKey {
				return nil, dbutil.ErrWrap("task_script.add.foreign_key", err, dbutil.ParamSummary("task_id", taskID), dbutil.ParamSummary("script_id", scriptID))
			}
		}
		return nil, dbutil.ErrWrap("task_script.add", err, dbutil.ParamSummary("task_id", taskID), dbutil.ParamSummary("script_id", scriptID), dbutil.ParamSummary("name", name))
	}
	return &ts, nil
}

// RemoveTaskScript detaches by name or alias. Returns rows affected.
func RemoveTaskScript(ctx context.Context, db *pgxpool.Pool, taskID, nameOrAlias string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM task_scripts WHERE task_id=$1::uuid AND (name=$2 OR alias=$2)`, taskID, nameOrAlias)
	if err != nil {
		return 0, dbutil.ErrWrap("task_script.remove", err, dbutil.ParamSummary("task_id", taskID), dbutil.ParamSummary("name_or_alias", nameOrAlias))
	}
	return ct.RowsAffected(), nil
}

// ListTaskScripts lists attachments for a task.
func ListTaskScripts(ctx context.Context, db *pgxpool.Pool, taskID string) ([]TaskScript, error) {
	rows, err := db.Query(ctx, `SELECT id::text, task_id::text, script_id::text, name, alias, created_at
                                 FROM task_scripts WHERE task_id=$1::uuid ORDER BY created_at ASC, name ASC`, taskID)
	if err != nil {
		return nil, dbutil.ErrWrap("task_script.list", err, dbutil.ParamSummary("task_id", taskID))
	}
	defer rows.Close()
	var out []TaskScript
	for rows.Next() {
		var ts TaskScript
		if err := rows.Scan(&ts.ID, &ts.TaskID, &ts.ScriptID, &ts.Name, &ts.Alias, &ts.CreatedAt); err != nil {
			return nil, dbutil.ErrWrap("task_script.list.scan", err)
		}
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("task_script.list", err)
	}
	return out, nil
}

// ResolveTaskScript returns the Script row associated to a task by name (preferred) or alias.
func ResolveTaskScript(ctx context.Context, db *pgxpool.Pool, taskID, nameOrAlias string) (*Script, error) {
	q := `SELECT s.id::text, s.title, s.description, s.motivation, s.notes,
                 encode(s.script_content_id,'hex'), s.role_name, s.tags, s.created, s.updated
          FROM task_scripts ts
          JOIN scripts s ON s.id = ts.script_id
          WHERE ts.task_id=$1::uuid AND (ts.name=$2 OR ts.alias=$2)
          ORDER BY CASE WHEN ts.name=$2 THEN 0 ELSE 1 END
          LIMIT 1`
	var sc Script
	var tagsJSON []byte
	if err := db.QueryRow(ctx, q, taskID, nameOrAlias).
		Scan(&sc.ID, &sc.Title, &sc.Description, &sc.Motivation, &sc.Notes, &sc.ScriptContentID, &sc.RoleName, &tagsJSON, &sc.Created, &sc.Updated); err != nil {
		return nil, dbutil.ErrWrap("task_script.resolve.not_found", err, dbutil.ParamSummary("task_id", taskID), dbutil.ParamSummary("name_or_alias", nameOrAlias))
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &sc.Tags)
	}
	return &sc, nil
}

// ListTasksUsingScript lists tasks that reference the given script via attachments.
func ListTasksUsingScript(ctx context.Context, db *pgxpool.Pool, scriptID string) ([]Task, error) {
	rows, err := db.Query(ctx, `SELECT t.id::text, tv.workflow_id, t.command, t.variant, t.title, t.description, t.motivation,
                                       t.notes, t.shell, t.timeout::text, t.tool_workspace_id::text, t.tags, t.level, t.archived, t.created
                                FROM task_scripts ts
                                JOIN tasks t ON t.id = ts.task_id
                                LEFT JOIN task_variants tv ON tv.variant = t.variant
                                WHERE ts.script_id=$1::uuid
                                ORDER BY t.created DESC`, scriptID)
	if err != nil {
		return nil, dbutil.ErrWrap("task_script.tasks.list", err, dbutil.ParamSummary("script_id", scriptID))
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		var tagsJSON []byte
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Command, &t.Variant, &t.Title, &t.Description, &t.Motivation,
			&t.Notes, &t.Shell, &t.Timeout, &t.ToolWorkspaceID, &tagsJSON, &t.Level, &t.Archived, &t.Created); err != nil {
			return nil, dbutil.ErrWrap("task_script.tasks.scan", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &t.Tags)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("task_script.tasks.list", err)
	}
	return out, nil
}

// end
