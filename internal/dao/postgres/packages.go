package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Package struct {
    ID        string
    RoleName  string
    Variant   string
    Version   string
    TaskID    string
    Created   sql.NullTime
    Updated   sql.NullTime
}

// UpsertStarredTask binds a role to a specific (variant, version) by referencing the task row.
// Enforces uniqueness on (role, variant) so later calls update the chosen version.
func UpsertPackage(ctx context.Context, db *pgxpool.Pool, roleName, variant, version string) (*Package, error) {
    // Resolve the task id for integrity
    t, err := GetTaskByKey(ctx, db, variant, version)
    if err != nil { return nil, err }
    q := `INSERT INTO packages (role_name, variant, version, task_id)
          VALUES ($1,$2,$3,$4::uuid)
          ON CONFLICT (role_name, variant) DO UPDATE SET
            version = EXCLUDED.version,
            task_id = EXCLUDED.task_id,
            updated = now()
          RETURNING id::text, created, updated`
    var p Package
    p.RoleName = roleName; p.Variant = variant; p.Version = version; p.TaskID = t.ID
    if err := db.QueryRow(ctx, q, roleName, variant, version, t.ID).Scan(&p.ID, &p.Created, &p.Updated); err != nil {
        return nil, err
    }
    return &p, nil
}

// GetStarredTaskByID fetches a starred task by id.
func GetPackageByID(ctx context.Context, db *pgxpool.Pool, id string) (*Package, error) {
    q := `SELECT id::text, role_name, variant, version, task_id::text, created, updated FROM packages WHERE id=$1::uuid`
    var p Package
    if err := db.QueryRow(ctx, q, id).Scan(&p.ID, &p.RoleName, &p.Variant, &p.Version, &p.TaskID, &p.Created, &p.Updated); err != nil {
        return nil, err
    }
    return &p, nil
}

// GetStarredTaskByKey fetches a starred task by (role, variant).
func GetPackageByKey(ctx context.Context, db *pgxpool.Pool, roleName, variant string) (*Package, error) {
    q := `SELECT id::text, role_name, variant, version, task_id::text, created, updated FROM packages WHERE role_name=$1 AND variant=$2`
    var p Package
    if err := db.QueryRow(ctx, q, roleName, variant).Scan(&p.ID, &p.RoleName, &p.Variant, &p.Version, &p.TaskID, &p.Created, &p.Updated); err != nil {
        return nil, err
    }
    return &p, nil
}

// ListStarredTasks lists starred tasks with optional filters.
func ListPackages(ctx context.Context, db *pgxpool.Pool, roleName, variant string, limit, offset int) ([]Package, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    switch {
    case stringsTrim(roleName) != "" && stringsTrim(variant) != "":
        rows, err = db.Query(ctx, `SELECT id::text, role_name, variant, version, task_id::text, created, updated FROM packages WHERE role_name=$1 AND variant=$2 ORDER BY role_name, variant LIMIT $3 OFFSET $4`, roleName, variant, limit, offset)
    case stringsTrim(roleName) != "":
        rows, err = db.Query(ctx, `SELECT id::text, role_name, variant, version, task_id::text, created, updated FROM packages WHERE role_name=$1 ORDER BY variant LIMIT $2 OFFSET $3`, roleName, limit, offset)
    case stringsTrim(variant) != "":
        rows, err = db.Query(ctx, `SELECT id::text, role_name, variant, version, task_id::text, created, updated FROM packages WHERE variant=$1 ORDER BY role_name LIMIT $2 OFFSET $3`, variant, limit, offset)
    default:
        rows, err = db.Query(ctx, `SELECT id::text, role_name, variant, version, task_id::text, created, updated FROM packages ORDER BY role_name, variant LIMIT $1 OFFSET $2`, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Package
    for rows.Next() {
        var p Package
        if err := rows.Scan(&p.ID, &p.RoleName, &p.Variant, &p.Version, &p.TaskID, &p.Created, &p.Updated); err != nil {
            return nil, err
        }
        out = append(out, p)
    }
    return out, rows.Err()
}

// DeleteStarredTaskByID deletes a starred task by id.
func DeletePackageByID(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM packages WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

// DeleteStarredTaskByKey deletes a starred task by (role, variant).
func DeletePackageByKey(ctx context.Context, db *pgxpool.Pool, roleName, variant string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM packages WHERE role_name=$1 AND variant=$2`, roleName, variant)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}
