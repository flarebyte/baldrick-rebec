package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type StarredTask struct {
    ID      int64
    Role    string
    Variant string
    Version string
    TaskID  int64
    Created sql.NullTime
    Updated sql.NullTime
}

// UpsertStarredTask binds a role to a specific (variant, version) by referencing the task row.
// Enforces uniqueness on (role, variant) so later calls update the chosen version.
func UpsertStarredTask(ctx context.Context, db *pgxpool.Pool, role, variant, version string) (*StarredTask, error) {
    // Resolve the task id for integrity
    t, err := GetTaskByKey(ctx, db, variant, version)
    if err != nil { return nil, err }
    q := `INSERT INTO starred_tasks (role, variant, version, task_id)
          VALUES ($1,$2,$3,$4)
          ON CONFLICT (role, variant) DO UPDATE SET
            version = EXCLUDED.version,
            task_id = EXCLUDED.task_id,
            updated = now()
          RETURNING id, created, updated`
    var st StarredTask
    st.Role = role; st.Variant = variant; st.Version = version; st.TaskID = t.ID
    if err := db.QueryRow(ctx, q, role, variant, version, t.ID).Scan(&st.ID, &st.Created, &st.Updated); err != nil {
        return nil, err
    }
    return &st, nil
}

// GetStarredTaskByID fetches a starred task by id.
func GetStarredTaskByID(ctx context.Context, db *pgxpool.Pool, id int64) (*StarredTask, error) {
    q := `SELECT id, role, variant, version, task_id, created, updated FROM starred_tasks WHERE id=$1`
    var st StarredTask
    if err := db.QueryRow(ctx, q, id).Scan(&st.ID, &st.Role, &st.Variant, &st.Version, &st.TaskID, &st.Created, &st.Updated); err != nil {
        return nil, err
    }
    return &st, nil
}

// GetStarredTaskByKey fetches a starred task by (role, variant).
func GetStarredTaskByKey(ctx context.Context, db *pgxpool.Pool, role, variant string) (*StarredTask, error) {
    q := `SELECT id, role, variant, version, task_id, created, updated FROM starred_tasks WHERE role=$1 AND variant=$2`
    var st StarredTask
    if err := db.QueryRow(ctx, q, role, variant).Scan(&st.ID, &st.Role, &st.Variant, &st.Version, &st.TaskID, &st.Created, &st.Updated); err != nil {
        return nil, err
    }
    return &st, nil
}

// ListStarredTasks lists starred tasks with optional filters.
func ListStarredTasks(ctx context.Context, db *pgxpool.Pool, role, variant string, limit, offset int) ([]StarredTask, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    switch {
    case stringsTrim(role) != "" && stringsTrim(variant) != "":
        rows, err = db.Query(ctx, `SELECT id, role, variant, version, task_id, created, updated FROM starred_tasks WHERE role=$1 AND variant=$2 ORDER BY role, variant LIMIT $3 OFFSET $4`, role, variant, limit, offset)
    case stringsTrim(role) != "":
        rows, err = db.Query(ctx, `SELECT id, role, variant, version, task_id, created, updated FROM starred_tasks WHERE role=$1 ORDER BY variant LIMIT $2 OFFSET $3`, role, limit, offset)
    case stringsTrim(variant) != "":
        rows, err = db.Query(ctx, `SELECT id, role, variant, version, task_id, created, updated FROM starred_tasks WHERE variant=$1 ORDER BY role LIMIT $2 OFFSET $3`, variant, limit, offset)
    default:
        rows, err = db.Query(ctx, `SELECT id, role, variant, version, task_id, created, updated FROM starred_tasks ORDER BY role, variant LIMIT $1 OFFSET $2`, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []StarredTask
    for rows.Next() {
        var st StarredTask
        if err := rows.Scan(&st.ID, &st.Role, &st.Variant, &st.Version, &st.TaskID, &st.Created, &st.Updated); err != nil {
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

// DeleteStarredTaskByKey deletes a starred task by (role, variant).
func DeleteStarredTaskByKey(ctx context.Context, db *pgxpool.Pool, role, variant string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM starred_tasks WHERE role=$1 AND variant=$2`, role, variant)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
