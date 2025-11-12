package postgres

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
)

type Workflow struct {
    Name        string
    Title       string
    Description sql.NullString
    Notes       sql.NullString
    Created     sql.NullTime
    Updated     sql.NullTime
}

// UpsertWorkflow inserts or updates a workflow by name.
func UpsertWorkflow(ctx context.Context, db *pgxpool.Pool, w *Workflow) error {
    q := `INSERT INTO workflows (name, title, description, notes)
          VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))
          ON CONFLICT (name) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            notes = EXCLUDED.notes,
            updated = now()
          RETURNING created, updated`
    if err := db.QueryRow(ctx, q, w.Name, w.Title, stringOrEmpty(w.Description), stringOrEmpty(w.Notes)).Scan(&w.Created, &w.Updated); err != nil {
        return dbutil.ErrWrap("workflow.upsert", err, dbutil.ParamSummary("name", w.Name))
    }
    return nil
}

// GetWorkflowByName fetches a workflow by its unique name.
func GetWorkflowByName(ctx context.Context, db *pgxpool.Pool, name string) (*Workflow, error) {
    q := `SELECT name, title, description, notes, created, updated FROM workflows WHERE name=$1`
    var w Workflow
    if err := db.QueryRow(ctx, q, name).Scan(&w.Name, &w.Title, &w.Description, &w.Notes, &w.Created, &w.Updated); err != nil {
        return nil, dbutil.ErrWrap("workflow.get", err, dbutil.ParamSummary("name", name))
    }
    return &w, nil
}

// ListWorkflows returns workflows ordered by name with optional pagination.
func ListWorkflows(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Workflow, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT name, title, description, notes, created, updated
          FROM workflows
          WHERE role_name=$1
          ORDER BY name ASC
          LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, dbutil.ErrWrap("workflow.list", err, dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset)) }
    defer rows.Close()
    var out []Workflow
    for rows.Next() {
        var w Workflow
        if err := rows.Scan(&w.Name, &w.Title, &w.Description, &w.Notes, &w.Created, &w.Updated); err != nil {
            return nil, dbutil.ErrWrap("workflow.list.scan", err)
        }
        out = append(out, w)
    }
    if err := rows.Err(); err != nil { return nil, dbutil.ErrWrap("workflow.list", err) }
    return out, nil
}

// DeleteWorkflow removes a workflow by name. Returns number of rows affected.
func DeleteWorkflow(ctx context.Context, db *pgxpool.Pool, name string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM workflows WHERE name=$1`, name)
    if err != nil { return 0, dbutil.ErrWrap("workflow.delete", err, dbutil.ParamSummary("name", name)) }
    return ct.RowsAffected(), nil
}
