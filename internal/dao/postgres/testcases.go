package postgres

import (
    "context"
    "database/sql"
    "encoding/json"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Testcase struct {
    ID            string
    Name          sql.NullString
    Package       sql.NullString
    Classname     sql.NullString
    Title         string
    ExperimentID  sql.NullString
    RoleName      string
    Status        string
    ErrorMessage  sql.NullString
    Tags          map[string]any
    Level         sql.NullString
    Created       sql.NullTime
    File          sql.NullString
    Line          sql.NullInt64
    ExecutionTime sql.NullFloat64
}

func InsertTestcase(ctx context.Context, db *pgxpool.Pool, t *Testcase) error {
    q := `INSERT INTO testcases (
            name, package, classname, title, experiment_id, role_name, status, error_message, tags, level, file, line, execution_time
          ) VALUES (
            NULLIF($1,''), NULLIF($2,''), NULLIF($3,''), $4,
            CASE WHEN $5='' THEN NULL ELSE $5::uuid END,
            COALESCE(NULLIF($6,''),'user'), COALESCE(NULLIF($7,''),'KO'),
            NULLIF($8,''), COALESCE($9,'{}'::jsonb), NULLIF($10,''), NULLIF($11,''), $12, $13
          ) RETURNING id::text, created`
    var tagsJSON []byte
    if t.Tags != nil { tagsJSON, _ = json.Marshal(t.Tags) }
    return db.QueryRow(ctx, q,
        stringOrEmpty(t.Name), stringOrEmpty(t.Package), stringOrEmpty(t.Classname), t.Title, stringOrEmpty(t.ExperimentID), t.RoleName, t.Status,
        stringOrEmpty(t.ErrorMessage), tagsJSON, stringOrEmpty(t.Level), stringOrEmpty(t.File), nullOrInt(t.Line), nullOrFloat(t.ExecutionTime),
    ).Scan(&t.ID, &t.Created)
}

func ListTestcases(ctx context.Context, db *pgxpool.Pool, roleName, experimentID, status string, limit, offset int) ([]Testcase, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    switch {
    case stringsTrim(experimentID) != "" && stringsTrim(status) != "":
        rows, err = db.Query(ctx, `SELECT id::text, name, package, classname, title, experiment_id::text, role_name, status, error_message, tags, level, created, file, line, execution_time
                                   FROM testcases WHERE role_name=$1 AND experiment_id=$2::uuid AND status=$3 ORDER BY created DESC LIMIT $4 OFFSET $5`, roleName, experimentID, status, limit, offset)
    case stringsTrim(experimentID) != "":
        rows, err = db.Query(ctx, `SELECT id::text, name, package, classname, title, experiment_id::text, role_name, status, error_message, tags, level, created, file, line, execution_time
                                   FROM testcases WHERE role_name=$1 AND experiment_id=$2::uuid ORDER BY created DESC LIMIT $3 OFFSET $4`, roleName, experimentID, limit, offset)
    case stringsTrim(status) != "":
        rows, err = db.Query(ctx, `SELECT id::text, name, package, classname, title, experiment_id::text, role_name, status, error_message, tags, level, created, file, line, execution_time
                                   FROM testcases WHERE role_name=$1 AND status=$2 ORDER BY created DESC LIMIT $3 OFFSET $4`, roleName, status, limit, offset)
    default:
        rows, err = db.Query(ctx, `SELECT id::text, name, package, classname, title, experiment_id::text, role_name, status, error_message, tags, level, created, file, line, execution_time
                                   FROM testcases WHERE role_name=$1 ORDER BY created DESC LIMIT $2 OFFSET $3`, roleName, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Testcase
    for rows.Next() {
        var t Testcase
        var tagsJSON []byte
        if err := rows.Scan(&t.ID, &t.Name, &t.Package, &t.Classname, &t.Title, &t.ExperimentID, &t.RoleName, &t.Status, &t.ErrorMessage, &tagsJSON, &t.Level, &t.Created, &t.File, &t.Line, &t.ExecutionTime); err != nil {
            return nil, err
        }
        if len(tagsJSON) > 0 { _ = json.Unmarshal(tagsJSON, &t.Tags) }
        out = append(out, t)
    }
    return out, rows.Err()
}

func DeleteTestcase(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM testcases WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

func nullOrInt(n sql.NullInt64) any { if n.Valid { return n.Int64 }; return nil }
func nullOrFloat(f sql.NullFloat64) any { if f.Valid { return f.Float64 }; return nil }
