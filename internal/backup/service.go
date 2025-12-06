package backup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options for creating a backup.
type BackupOptions struct {
	Schema      string         // backup schema name, default "backup"
	Description string         // optional description
	Tags        map[string]any // free-form tags
	InitiatedBy string         // creator
	Include     []string       // whitelist entity names
	Exclude     []string       // blacklist entity names
}

// CreateBackup orchestrates capturing schemas and records into the backup schema.
func CreateBackup(ctx context.Context, db *pgxpool.Pool, entities []BackupEntityConfig, opt BackupOptions) (string, error) {
	schema := opt.Schema
	if schema == "" {
		schema = "backup"
	}
	if err := pgdao.EnsureBackupSchema(ctx, db, schema); err != nil {
		return "", dbutil.ErrWrap("snapshot.ensure_backup_schema", err, dbutil.ParamSummary("schema", schema))
	}
	var desc *string
	if strings.TrimSpace(opt.Description) != "" {
		v := opt.Description
		desc = &v
	}
	var initiatedBy *string
	if strings.TrimSpace(opt.InitiatedBy) != "" {
		v := opt.InitiatedBy
		initiatedBy = &v
	}
	bID, err := pgdao.InsertBackup(ctx, db, schema, desc, opt.Tags, initiatedBy, nil)
	if err != nil {
		return "", dbutil.ErrWrap("snapshot.backup.create", err, dbutil.ParamSummary("schema", schema))
	}

	includeSet := setFromSlice(opt.Include)
	excludeSet := setFromSlice(opt.Exclude)

	for _, e := range entities {
		include := e.IncludeByDefault
		if len(includeSet) > 0 {
			_, include = includeSet[strings.ToLower(e.EntityName)]
		}
		if _, ex := excludeSet[strings.ToLower(e.EntityName)]; ex {
			include = false
		}
		if !include {
			continue
		}

		cols, err := fetchTableColumns(ctx, db, "public", e.TableName)
		if err != nil {
			return "", dbutil.ErrWrap("snapshot.fetch_columns", err, dbutil.ParamSummary("entity", e.EntityName), dbutil.ParamSummary("table", e.TableName))
		}
		// insert schema rows
		for _, c := range cols {
			def := c.ColumnDefault
			var defp *string
			if def != "" {
				defp = &def
			}
			if err := pgdao.InsertEntitySchema(ctx, db, schema, bID, e.EntityName, c.ColumnName, c.DataType, c.IsNullable, defp, nil); err != nil {
				return "", dbutil.ErrWrap("snapshot.entity_schema.insert", err,
					dbutil.ParamSummary("entity", e.EntityName), dbutil.ParamSummary("field", c.ColumnName))
			}
		}
		// build and stream records
		// whitelist PKs must exist in the table
		pk := make([]string, 0, len(e.PKColumns))
		tableCols := make(map[string]bool, len(cols))
		for _, c := range cols {
			tableCols[c.ColumnName] = true
		}
		for _, k := range e.PKColumns {
			if !tableCols[k] {
				return "", fmt.Errorf("table %s missing PK column %q", e.TableName, k)
			}
			pk = append(pk, k)
		}
		// prepare dynamic SQL
		// SELECT jsonb_build_object('k1', t.k1, ...), to_jsonb(t), role_name FROM public.table t
		// Detect role_name presence via discovered columns if not declared
		hasRole := e.HasRoleName && tableCols["role_name"]
		pkPairs := make([]string, 0, len(pk)*2)
		for _, k := range pk {
			pkPairs = append(pkPairs, fmt.Sprintf("'%s', t.%s", k, pgx.Identifier{k}.Sanitize()))
		}
		idf := pgx.Identifier{"public", e.TableName}
		q := fmt.Sprintf("SELECT jsonb_build_object(%s), to_jsonb(t)%s FROM %s AS t",
			strings.Join(pkPairs, ", "),
			ternary(hasRole, ", t.role_name", ""),
			idf.Sanitize(),
		)
		rows, err := db.Query(ctx, q)
		if err != nil {
			return "", dbutil.ErrWrap("snapshot.select_rows", err, dbutil.ParamSummary("table", e.TableName))
		}
		for rows.Next() {
			var pkb, rec []byte
			var role *string
			if hasRole {
				var r string
				if err := rows.Scan(&pkb, &rec, &r); err != nil {
					rows.Close()
					return "", dbutil.ErrWrap("snapshot.select_rows.scan", err, dbutil.ParamSummary("table", e.TableName))
				}
				role = &r
			} else {
				if err := rows.Scan(&pkb, &rec); err != nil {
					rows.Close()
					return "", dbutil.ErrWrap("snapshot.select_rows.scan", err, dbutil.ParamSummary("table", e.TableName))
				}
			}
			if err := pgdao.InsertEntityRecord(ctx, db, schema, bID, e.EntityName, pkb, rec, role); err != nil {
				rows.Close()
				return "", dbutil.ErrWrap("snapshot.entity_record.insert", err, dbutil.ParamSummary("entity", e.EntityName), dbutil.ParamSummary("schema", schema))
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return "", dbutil.ErrWrap("snapshot.select_rows", err, dbutil.ParamSummary("table", e.TableName))
		}
		rows.Close()
	}
	return bID, nil
}

// Column describes a column in information_schema.columns
type Column struct {
	ColumnName    string
	DataType      string
	IsNullable    bool
	ColumnDefault string
}

func fetchTableColumns(ctx context.Context, db *pgxpool.Pool, schema, table string) ([]Column, error) {
	q := `SELECT column_name, data_type, is_nullable, COALESCE(column_default,'')
          FROM information_schema.columns
          WHERE table_schema=$1 AND table_name=$2
          ORDER BY ordinal_position`
	rows, err := db.Query(ctx, q, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Column
	for rows.Next() {
		var c Column
		var nullable string
		if err := rows.Scan(&c.ColumnName, &c.DataType, &nullable, &c.ColumnDefault); err != nil {
			return nil, err
		}
		c.IsNullable = strings.EqualFold(nullable, "YES")
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("table %s.%s not found", schema, table)
	}
	return out, nil
}

// RestoreOptions defines restore behavior.
type RestoreOptions struct {
	Schema   string   // backup schema
	Entities []string // subset to restore; empty = all
	Mode     string   // replace|append
	DryRun   bool
}

// RestoreFromBackup restores records from a backup into live tables.
// Replace: TRUNCATE then insert. Append: insert missing PKs (ON CONFLICT DO NOTHING).
func RestoreFromBackup(ctx context.Context, db *pgxpool.Pool, entities []BackupEntityConfig, backupID string, opt RestoreOptions) error {
	schema := opt.Schema
	if schema == "" {
		schema = "backup"
	}
	emap := map[string]BackupEntityConfig{}
	for _, e := range entities {
		emap[strings.ToLower(e.EntityName)] = e
	}
	var targets []BackupEntityConfig
	if len(opt.Entities) == 0 {
		// restore all that exist in backup
		// list distinct entity names from entity_records
		rows, err := db.Query(ctx, `SELECT DISTINCT entity_name FROM `+schema+`.entity_records WHERE backup_id=$1`, backupID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return err
			}
			if e, ok := emap[strings.ToLower(n)]; ok {
				targets = append(targets, e)
			}
		}
		rows.Close()
	} else {
		for _, n := range opt.Entities {
			if e, ok := emap[strings.ToLower(n)]; ok {
				targets = append(targets, e)
			}
		}
	}
	// deterministic order by name
	sort.Slice(targets, func(i, j int) bool { return targets[i].EntityName < targets[j].EntityName })

	for _, e := range targets {
		liveCols, err := fetchTableColumns(ctx, db, "public", e.TableName)
		if err != nil {
			return dbutil.ErrWrap("snapshot.restore.fetch_columns", err, dbutil.ParamSummary("table", e.TableName))
		}
		// prepare column list excluding generated defaults if desired; include most columns
		// We will map from JSON record to explicit values.
		cols := make([]Column, 0, len(liveCols))
		for _, c := range liveCols {
			cols = append(cols, c)
		}
		// Replace mode: truncate
		if strings.EqualFold(opt.Mode, "replace") && !opt.DryRun {
			idf := pgx.Identifier{"public", e.TableName}
			if _, err := db.Exec(ctx, "TRUNCATE "+idf.Sanitize()+" RESTART IDENTITY CASCADE"); err != nil {
				return dbutil.ErrWrap("snapshot.restore.truncate", err, dbutil.ParamSummary("table", e.TableName))
			}
		}
		// Build INSERT ... SELECT from reading backup.entity_records
		// We'll pull record JSON and unmarshal in Go to map[string]any — then param insert.
		rows, err := db.Query(ctx, `SELECT record FROM `+schema+`.entity_records WHERE backup_id=$1 AND entity_name=$2`, backupID, e.EntityName)
		if err != nil {
			return dbutil.ErrWrap("snapshot.restore.read_records", err, dbutil.ParamSummary("entity", e.EntityName), dbutil.ParamSummary("schema", schema))
		}
		for rows.Next() {
			var recb []byte
			if err := rows.Scan(&recb); err != nil {
				rows.Close()
				return dbutil.ErrWrap("snapshot.restore.read_records.scan", err, dbutil.ParamSummary("entity", e.EntityName))
			}
			if opt.DryRun {
				continue
			}
			// decode
			var rec map[string]any
			if err := json.Unmarshal(recb, &rec); err != nil {
				rows.Close()
				return dbutil.ErrWrap("snapshot.restore.decode", err, dbutil.ParamSummary("entity", e.EntityName))
			}
			// Build column list and params present in rec
			var names []string
			var placeholders []string
			var args []any
			argn := 1
			for _, c := range cols {
				name := c.ColumnName
				v, ok := rec[name]
				if !ok {
					continue
				}
				names = append(names, pgx.Identifier{name}.Sanitize())
				// jsonb cast for jsonb columns
				ph := fmt.Sprintf("$%d", argn)
				if strings.EqualFold(c.DataType, "jsonb") {
					ph = ph + "::jsonb"
					// ensure value is marshaled back to json
					switch v.(type) {
					case map[string]any, []any:
						b, _ := json.Marshal(v)
						v = b
					}
				}
				placeholders = append(placeholders, ph)
				args = append(args, v)
				argn++
			}
			if len(names) == 0 {
				continue
			}
			idf := pgx.Identifier{"public", e.TableName}
			// ON CONFLICT handling for append
			onConflict := ""
			if strings.EqualFold(opt.Mode, "append") {
				if len(e.PKColumns) == 0 {
					rows.Close()
					return errors.New("append mode requires PKColumns")
				}
				var pkid []string
				for _, p := range e.PKColumns {
					pkid = append(pkid, pgx.Identifier{p}.Sanitize())
				}
				onConflict = fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", strings.Join(pkid, ", "))
			}
			q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)%s",
				idf.Sanitize(),
				strings.Join(names, ", "),
				strings.Join(placeholders, ", "),
				onConflict,
			)
			if _, err := db.Exec(ctx, q, args...); err != nil {
				rows.Close()
				return dbutil.ErrWrap("snapshot.restore.insert", err, dbutil.ParamSummary("table", e.TableName))
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return dbutil.ErrWrap("snapshot.restore.read_records", err, dbutil.ParamSummary("entity", e.EntityName))
		}
		rows.Close()
	}
	return nil
}

func setFromSlice(ss []string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, s := range ss {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" {
			m[s] = struct{}{}
		}
	}
	return m
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
