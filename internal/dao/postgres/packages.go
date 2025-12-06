package postgres

import (
	"context"
	"database/sql"
	"fmt"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Package struct {
	ID       string
	RoleName string
	TaskID   string
	Created  sql.NullTime
	Updated  sql.NullTime
}

// UpsertStarredTask binds a role to a specific (variant, version) by referencing the task row.
// Enforces uniqueness on (role, variant) so later calls update the chosen version.
func UpsertPackage(ctx context.Context, db *pgxpool.Pool, roleName, variant string) (*Package, error) {
	// Resolve the task id for integrity by variant
	t, err := GetTaskByVariant(ctx, db, variant)
	if err != nil {
		return nil, dbutil.ErrWrap("package.resolve_task", err, dbutil.ParamSummary("variant", variant))
	}
	q := `INSERT INTO packages (role_name, task_id)
          VALUES ($1,$2::uuid)
          ON CONFLICT (role_name, task_id) DO UPDATE SET
            task_id = EXCLUDED.task_id,
            updated = now()
          RETURNING id::text, created, updated`
	var p Package
	p.RoleName = roleName
	p.TaskID = t.ID
	if err := db.QueryRow(ctx, q, roleName, t.ID).Scan(&p.ID, &p.Created, &p.Updated); err != nil {
		return nil, dbutil.ErrWrap("package.upsert", err, dbutil.ParamSummary("role", roleName), dbutil.ParamSummary("task_id", t.ID))
	}
	return &p, nil
}

// GetStarredTaskByID fetches a starred task by id.
func GetPackageByID(ctx context.Context, db *pgxpool.Pool, id string) (*Package, error) {
	q := `SELECT id::text, role_name, task_id::text, created, updated FROM packages WHERE id=$1::uuid`
	var p Package
	if err := db.QueryRow(ctx, q, id).Scan(&p.ID, &p.RoleName, &p.TaskID, &p.Created, &p.Updated); err != nil {
		return nil, dbutil.ErrWrap("package.get", err, dbutil.ParamSummary("id", id))
	}
	return &p, nil
}

// GetStarredTaskByKey fetches a starred task by (role, variant).
func GetPackageByKey(ctx context.Context, db *pgxpool.Pool, roleName, variant string) (*Package, error) {
	// Join with tasks to filter by variant
	q := `SELECT p.id::text, p.role_name, p.task_id::text, p.created, p.updated
          FROM packages p
          JOIN tasks t ON t.id = p.task_id
          WHERE p.role_name=$1 AND t.variant=$2`
	var p Package
	if err := db.QueryRow(ctx, q, roleName, variant).Scan(&p.ID, &p.RoleName, &p.TaskID, &p.Created, &p.Updated); err != nil {
		return nil, dbutil.ErrWrap("package.get", err, dbutil.ParamSummary("role", roleName), dbutil.ParamSummary("variant", variant))
	}
	return &p, nil
}

// ListStarredTasks lists starred tasks with optional filters.
func ListPackages(ctx context.Context, db *pgxpool.Pool, roleName, variant string, limit, offset int) ([]Package, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var rows pgxRows
	var err error
	if stringsTrim(roleName) != "" && stringsTrim(variant) != "" {
		rows, err = db.Query(ctx, `SELECT p.id::text, p.role_name, p.task_id::text, p.created, p.updated
                                    FROM packages p JOIN tasks t ON t.id = p.task_id
                                    WHERE p.role_name=$1 AND t.variant=$2
                                    ORDER BY t.variant ASC LIMIT $3 OFFSET $4`, roleName, variant, limit, offset)
	} else if stringsTrim(roleName) != "" {
		rows, err = db.Query(ctx, `SELECT p.id::text, p.role_name, p.task_id::text, p.created, p.updated
                                    FROM packages p JOIN tasks t ON t.id = p.task_id
                                    WHERE p.role_name=$1
                                    ORDER BY t.variant ASC LIMIT $2 OFFSET $3`, roleName, limit, offset)
	} else if stringsTrim(variant) != "" {
		rows, err = db.Query(ctx, `SELECT p.id::text, p.role_name, p.task_id::text, p.created, p.updated
                                    FROM packages p JOIN tasks t ON t.id = p.task_id
                                    WHERE t.variant=$1
                                    ORDER BY t.variant ASC LIMIT $2 OFFSET $3`, variant, limit, offset)
	} else {
		rows, err = db.Query(ctx, `SELECT p.id::text, p.role_name, p.task_id::text, p.created, p.updated
                                    FROM packages p JOIN tasks t ON t.id = p.task_id
                                    ORDER BY t.variant ASC LIMIT $1 OFFSET $2`, limit, offset)
	}
	if err != nil {
		return nil, dbutil.ErrWrap("package.list", err, dbutil.ParamSummary("role", roleName), dbutil.ParamSummary("variant", variant), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.ID, &p.RoleName, &p.TaskID, &p.Created, &p.Updated); err != nil {
			return nil, dbutil.ErrWrap("package.list.scan", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("package.list", err)
	}
	return out, nil
}

// DeleteStarredTaskByID deletes a starred task by id.
func DeletePackageByID(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM packages WHERE id=$1::uuid`, id)
	if err != nil {
		return 0, dbutil.ErrWrap("package.delete", err, dbutil.ParamSummary("id", id))
	}
	return ct.RowsAffected(), nil
}

// DeleteStarredTaskByKey deletes a starred task by (role, variant).
func DeletePackageByKey(ctx context.Context, db *pgxpool.Pool, roleName, variant string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM packages p USING tasks t WHERE p.task_id=t.id AND p.role_name=$1 AND t.variant=$2`, roleName, variant)
	if err != nil {
		return 0, dbutil.ErrWrap("package.delete", err, dbutil.ParamSummary("role", roleName), dbutil.ParamSummary("variant", variant))
	}
	return ct.RowsAffected(), nil
}
