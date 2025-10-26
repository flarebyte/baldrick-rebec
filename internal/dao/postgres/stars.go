package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type StarredTask struct {
    ID      int64
    Mode    string
    Variant string
    Version string
    TaskID  int64
    Created sql.NullTime
    Updated sql.NullTime
}

// UpsertStarredTask binds a mode to a specific (variant, version) by referencing the task row.
// Enforces uniqueness on (mode, variant) so later calls update the chosen version.
func UpsertStarredTask(ctx context.Context, db *pgxpool.Pool, mode, variant, version string) (*StarredTask, error) {
    // Resolve the task id for integrity
    t, err := GetTaskByKey(ctx, db, variant, version)
    if err != nil { return nil, err }
    q := `INSERT INTO starred_tasks (mode, variant, version, task_id)
          VALUES ($1,$2,$3,$4)
          ON CONFLICT (mode, variant) DO UPDATE SET
            version = EXCLUDED.version,
            task_id = EXCLUDED.task_id,
            updated = now()
          RETURNING id, created, updated`
    var st StarredTask
    st.Mode = mode; st.Variant = variant; st.Version = version; st.TaskID = t.ID
    if err := db.QueryRow(ctx, q, mode, variant, version, t.ID).Scan(&st.ID, &st.Created, &st.Updated); err != nil {
        return nil, err
    }
    return &st, nil
}

// GetStarredTaskByID fetches a starred task by id.
func GetStarredTaskByID(ctx context.Context, db *pgxpool.Pool, id int64) (*StarredTask, error) {
    q := `SELECT id, mode, variant, version, task_id, created, updated FROM starred_tasks WHERE id=$1`
    var st StarredTask
    if err := db.QueryRow(ctx, q, id).Scan(&st.ID, &st.Mode, &st.Variant, &st.Version, &st.TaskID, &st.Created, &st.Updated); err != nil {
        return nil, err
    }
    return &st, nil
}

// GetStarredTaskByKey fetches a starred task by (mode, variant).
func GetStarredTaskByKey(ctx context.Context, db *pgxpool.Pool, mode, variant string) (*StarredTask, error) {
    q := `SELECT id, mode, variant, version, task_id, created, updated FROM starred_tasks WHERE mode=$1 AND variant=$2`
    var st StarredTask
    if err := db.QueryRow(ctx, q, mode, variant).Scan(&st.ID, &st.Mode, &st.Variant, &st.Version, &st.TaskID, &st.Created, &st.Updated); err != nil {
        return nil, err
    }
    return &st, nil
}

// ListStarredTasks lists starred tasks with optional filters.
func ListStarredTasks(ctx context.Context, db *pgxpool.Pool, mode, variant string, limit, offset int) ([]StarredTask, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    switch {
    case stringsTrim(mode) != "" && stringsTrim(variant) != "":
        rows, err = db.Query(ctx, `SELECT id, mode, variant, version, task_id, created, updated FROM starred_tasks WHERE mode=$1 AND variant=$2 ORDER BY mode, variant LIMIT $3 OFFSET $4`, mode, variant, limit, offset)
    case stringsTrim(mode) != "":
        rows, err = db.Query(ctx, `SELECT id, mode, variant, version, task_id, created, updated FROM starred_tasks WHERE mode=$1 ORDER BY variant LIMIT $2 OFFSET $3`, mode, limit, offset)
    case stringsTrim(variant) != "":
        rows, err = db.Query(ctx, `SELECT id, mode, variant, version, task_id, created, updated FROM starred_tasks WHERE variant=$1 ORDER BY mode LIMIT $2 OFFSET $3`, variant, limit, offset)
    default:
        rows, err = db.Query(ctx, `SELECT id, mode, variant, version, task_id, created, updated FROM starred_tasks ORDER BY mode, variant LIMIT $1 OFFSET $2`, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []StarredTask
    for rows.Next() {
        var st StarredTask
        if err := rows.Scan(&st.ID, &st.Mode, &st.Variant, &st.Version, &st.TaskID, &st.Created, &st.Updated); err != nil {
            return nil, err
        }
        out = append(out, st)
    }
    return out, rows.Err()
}

// DeleteStarredTaskByID deletes a starred task by id.
func DeleteStarredTaskByID(ctx context.Context, db *pgxpool.Pool, id int64) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM starred_tasks WHERE id=$1`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

// DeleteStarredTaskByKey deletes a starred task by (mode, variant).
func DeleteStarredTaskByKey(ctx context.Context, db *pgxpool.Pool, mode, variant string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM starred_tasks WHERE mode=$1 AND variant=$2`, mode, variant)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
