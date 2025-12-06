package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Tool struct {
	Name        string
	Title       string
	Description sql.NullString
	RoleName    string
	Created     sql.NullTime
	Updated     sql.NullTime
	Notes       sql.NullString
	Tags        map[string]any
	Settings    map[string]any
	ToolType    sql.NullString
}

// UpsertTool inserts or updates a tool identified by name (role-scoped via role_name column).
func UpsertTool(ctx context.Context, db *pgxpool.Pool, t *Tool) error {
	q := `INSERT INTO tools (name, title, description, role_name, notes, tags, settings, tool_type)
          VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,''), COALESCE($6,'{}'::jsonb), COALESCE($7,'{}'::jsonb), NULLIF($8,''))
          ON CONFLICT (name) DO UPDATE SET
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            role_name = EXCLUDED.role_name,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            settings = EXCLUDED.settings,
            tool_type = EXCLUDED.tool_type,
            updated = now()
          RETURNING created, updated`
	var tagsJSON, settingsJSON []byte
	if t.Tags != nil {
		tagsJSON, _ = json.Marshal(t.Tags)
	}
	if t.Settings != nil {
		settingsJSON, _ = json.Marshal(t.Settings)
	}
	if err := db.QueryRow(ctx, q,
		t.Name, t.Title, stringOrEmpty(t.Description), t.RoleName, stringOrEmpty(t.Notes), string(tagsJSON), string(settingsJSON), stringOrEmpty(t.ToolType),
	).Scan(&t.Created, &t.Updated); err != nil {
		return dbutil.ErrWrap("tool.upsert", err, dbutil.ParamSummary("name", t.Name), dbutil.ParamSummary("role", t.RoleName))
	}
	return nil
}

// GetToolByName fetches a tool by name.
func GetToolByName(ctx context.Context, db *pgxpool.Pool, name string) (*Tool, error) {
	q := `SELECT name, title, description, role_name, created, updated, notes, tags, settings, tool_type
          FROM tools WHERE name=$1`
	var t Tool
	var tagsJSON, settingsJSON []byte
	if err := db.QueryRow(ctx, q, name).Scan(&t.Name, &t.Title, &t.Description, &t.RoleName, &t.Created, &t.Updated, &t.Notes, &tagsJSON, &settingsJSON, &t.ToolType); err != nil {
		return nil, dbutil.ErrWrap("tool.get", err, dbutil.ParamSummary("name", name))
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &t.Tags)
	}
	if len(settingsJSON) > 0 {
		_ = json.Unmarshal(settingsJSON, &t.Settings)
	}
	return &t, nil
}

// ListTools lists tools filtered by role (required) with pagination.
func ListTools(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Tool, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT name, title, description, role_name, created, updated, notes, tags, settings, tool_type
          FROM tools WHERE role_name=$1 ORDER BY name ASC LIMIT $2 OFFSET $3`
	rows, err := db.Query(ctx, q, roleName, limit, offset)
	if err != nil {
		return nil, dbutil.ErrWrap("tool.list", err, dbutil.ParamSummary("role", roleName), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []Tool
	for rows.Next() {
		var t Tool
		var tagsJSON, settingsJSON []byte
		if err := rows.Scan(&t.Name, &t.Title, &t.Description, &t.RoleName, &t.Created, &t.Updated, &t.Notes, &tagsJSON, &settingsJSON, &t.ToolType); err != nil {
			return nil, dbutil.ErrWrap("tool.list.scan", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &t.Tags)
		}
		if len(settingsJSON) > 0 {
			_ = json.Unmarshal(settingsJSON, &t.Settings)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("tool.list", err)
	}
	return out, nil
}

// DeleteTool removes a tool by name.
func DeleteTool(ctx context.Context, db *pgxpool.Pool, name string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM tools WHERE name=$1`, name)
	if err != nil {
		return 0, dbutil.ErrWrap("tool.delete", err, dbutil.ParamSummary("name", name))
	}
	return ct.RowsAffected(), nil
}
