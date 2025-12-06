package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Project struct {
	Name        string
	RoleName    string
	Description sql.NullString
	Notes       sql.NullString
	Tags        map[string]any
	Created     sql.NullTime
	Updated     sql.NullTime
}

// UpsertProject inserts or updates a project identified by (name, role_name).
func UpsertProject(ctx context.Context, db *pgxpool.Pool, p *Project) error {
	q := `INSERT INTO projects (name, role_name, description, notes, tags)
          VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), COALESCE($5,'{}'::jsonb))
          ON CONFLICT (name, role_name) DO UPDATE SET
            description = EXCLUDED.description,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            updated = now()
          RETURNING created, updated`
	var tagsJSON []byte
	if p.Tags != nil {
		tagsJSON, _ = json.Marshal(p.Tags)
	}
	if err := db.QueryRow(ctx, q,
		p.Name, p.RoleName, stringOrEmpty(p.Description), stringOrEmpty(p.Notes), tagsJSON,
	).Scan(&p.Created, &p.Updated); err != nil {
		return dbutil.ErrWrap("project.upsert", err, dbutil.ParamSummary("name", p.Name), dbutil.ParamSummary("role", p.RoleName))
	}
	return nil
}

// GetProjectByKey fetches a project by (name, role_name).
func GetProjectByKey(ctx context.Context, db *pgxpool.Pool, name, roleName string) (*Project, error) {
	q := `SELECT name, role_name, description, notes, tags, created, updated
          FROM projects WHERE name=$1 AND role_name=$2`
	var p Project
	var tagsJSON []byte
	if err := db.QueryRow(ctx, q, name, roleName).Scan(&p.Name, &p.RoleName, &p.Description, &p.Notes, &tagsJSON, &p.Created, &p.Updated); err != nil {
		return nil, dbutil.ErrWrap("project.get", err, dbutil.ParamSummary("name", name), dbutil.ParamSummary("role", roleName))
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &p.Tags)
	}
	return &p, nil
}

// ListProjects returns projects for a role ordered by name.
func ListProjects(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Project, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT name, role_name, description, notes, tags, created, updated
          FROM projects WHERE role_name=$1 ORDER BY name ASC LIMIT $2 OFFSET $3`
	rows, err := db.Query(ctx, q, roleName, limit, offset)
	if err != nil {
		return nil, dbutil.ErrWrap("project.list", err, dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var tagsJSON []byte
		if err := rows.Scan(&p.Name, &p.RoleName, &p.Description, &p.Notes, &tagsJSON, &p.Created, &p.Updated); err != nil {
			return nil, dbutil.ErrWrap("project.list.scan", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &p.Tags)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("project.list", err)
	}
	return out, nil
}

// DeleteProject removes a project by (name, role_name).
func DeleteProject(ctx context.Context, db *pgxpool.Pool, name, roleName string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM projects WHERE name=$1 AND role_name=$2`, name, roleName)
	if err != nil {
		return 0, dbutil.ErrWrap("project.delete", err, dbutil.ParamSummary("name", name), dbutil.ParamSummary("role", roleName))
	}
	return ct.RowsAffected(), nil
}
