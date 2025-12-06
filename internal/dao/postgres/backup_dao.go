package postgres

import (
	"context"
	"time"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strconv"
)

// Backup represents a row in backup.backups.
type Backup struct {
	ID             string         `json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	Description    *string        `json:"description,omitempty"`
	Tags           map[string]any `json:"tags"`
	InitiatedBy    *string        `json:"initiated_by,omitempty"`
	RetentionUntil *time.Time     `json:"retention_until,omitempty"`
}

// InsertBackup inserts a new backup entry and returns its ID.
func InsertBackup(ctx context.Context, db *pgxpool.Pool, schema string, description *string, tags map[string]any, initiatedBy *string, retention *time.Time) (string, error) {
	if schema == "" {
		schema = "backup"
	}
	var id string
	// tags -> jsonb
	var tagsJSON any = nil
	if tags != nil {
		tagsJSON = tags
	}
	q := `INSERT INTO ` + schema + `.backups (description, tags, initiated_by, retention_until)
          VALUES ($1, $2, $3, $4) RETURNING id`
	if err := db.QueryRow(ctx, q, description, tagsJSON, initiatedBy, retention).Scan(&id); err != nil {
		return "", dbutil.ErrWrap("backup.insert", err, dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("desc", description))
	}
	return id, nil
}

// InsertEntitySchema inserts one entity schema row.
func InsertEntitySchema(ctx context.Context, db *pgxpool.Pool, schema string, backupID string, entityName string, fieldName string, fieldType string, isNullable bool, defaultValue *string, metadata any) error {
	if schema == "" {
		schema = "backup"
	}
	q := `INSERT INTO ` + schema + `.entity_schema (backup_id, entity_name, field_name, field_type, is_nullable, default_value, metadata)
          VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := db.Exec(ctx, q, backupID, entityName, fieldName, fieldType, isNullable, defaultValue, metadata)
	return dbutil.ErrWrap("backup.entity_schema.insert", err,
		dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("entity", entityName), dbutil.ParamSummary("field", fieldName))
}

// InsertEntityRecord inserts one entity record snapshot.
func InsertEntityRecord(ctx context.Context, db *pgxpool.Pool, schema string, backupID string, entityName string, recordPK []byte, record []byte, roleName *string) error {
	if schema == "" {
		schema = "backup"
	}
	q := `INSERT INTO ` + schema + `.entity_records (backup_id, entity_name, record_pk, record, role_name)
          VALUES ($1, $2, $3::jsonb, $4::jsonb, $5)`
	// Cast parameters to text for ::jsonb
	var pkS, recS string
	if recordPK != nil {
		pkS = string(recordPK)
	}
	if record != nil {
		recS = string(record)
	}
	_, err := db.Exec(ctx, q, backupID, entityName, pkS, recS, roleName)
	return dbutil.ErrWrap("backup.entity_record.insert", err,
		dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("entity", entityName), dbutil.ParamSummary("role", roleName))
}

// ListBackups returns backups filtered by time range and limited.
func ListBackups(ctx context.Context, db *pgxpool.Pool, schema string, since, until *time.Time, limit int) ([]Backup, error) {
	if schema == "" {
		schema = "backup"
	}
	q := `SELECT id, created_at, description, tags, initiated_by, retention_until
          FROM ` + schema + `.backups WHERE 1=1`
	args := []any{}
	if since != nil {
		q += ` AND created_at >= $` + itoa(len(args)+1)
		args = append(args, *since)
	}
	if until != nil {
		q += ` AND created_at <= $` + itoa(len(args)+1)
		args = append(args, *until)
	}
	q += ` ORDER BY created_at DESC`
	if limit > 0 {
		q += ` LIMIT $` + itoa(len(args)+1)
		args = append(args, limit)
	}
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, dbutil.ErrWrap("backup.list", err, dbutil.ParamSummary("schema", schema))
	}
	defer rows.Close()
	var out []Backup
	for rows.Next() {
		var b Backup
		var tags map[string]any
		if err := rows.Scan(&b.ID, &b.CreatedAt, &b.Description, &tags, &b.InitiatedBy, &b.RetentionUntil); err != nil {
			return nil, err
		}
		b.Tags = tags
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("backup.list", err, dbutil.ParamSummary("schema", schema))
	}
	return out, nil
}

// DeleteBackup deletes a backup by id (cascade).
func DeleteBackup(ctx context.Context, db *pgxpool.Pool, schema, id string) (int64, error) {
	if schema == "" {
		schema = "backup"
	}
	ct, err := db.Exec(ctx, `DELETE FROM `+schema+`.backups WHERE id=$1`, id)
	if err != nil {
		return 0, dbutil.ErrWrap("backup.delete", err, dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("id", id))
	}
	return ct.RowsAffected(), nil
}

// CountPerEntity returns record counts per entity for a backup.
func CountPerEntity(ctx context.Context, db *pgxpool.Pool, schema, backupID string) (map[string]int64, error) {
	if schema == "" {
		schema = "backup"
	}
	rows, err := db.Query(ctx, `SELECT entity_name, COUNT(*) FROM `+schema+`.entity_records WHERE backup_id=$1 GROUP BY entity_name ORDER BY entity_name`, backupID)
	if err != nil {
		return nil, dbutil.ErrWrap("backup.count_per_entity", err, dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("id", backupID))
	}
	defer rows.Close()
	m := map[string]int64{}
	for rows.Next() {
		var name string
		var c int64
		if err := rows.Scan(&name, &c); err != nil {
			return nil, err
		}
		m[name] = c
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("backup.count_per_entity", err, dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("id", backupID))
	}
	return m, nil
}

// ListBackupEntities returns the distinct entity names present for a backup.
func ListBackupEntities(ctx context.Context, db *pgxpool.Pool, schema, backupID string) ([]string, error) {
	if schema == "" {
		schema = "backup"
	}
	rows, err := db.Query(ctx, `SELECT DISTINCT entity_name FROM `+schema+`.entity_records WHERE backup_id=$1 ORDER BY entity_name`, backupID)
	if err != nil {
		return nil, dbutil.ErrWrap("backup.entities", err, dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("id", backupID))
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, dbutil.ErrWrap("backup.entities.scan", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("backup.entities", err)
	}
	return out, nil
}

// CountBackupEntityRecords returns the number of records for an entity in a backup.
func CountBackupEntityRecords(ctx context.Context, db *pgxpool.Pool, schema, backupID, entity string) (int64, error) {
	if schema == "" {
		schema = "backup"
	}
	var n int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM `+schema+`.entity_records WHERE backup_id=$1 AND entity_name=$2`, backupID, entity).Scan(&n)
	if err != nil {
		return 0, dbutil.ErrWrap("backup.entity.count", err, dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("entity", entity))
	}
	return n, nil
}

// CountTable counts rows in a given schema-qualified live table.
func CountTable(ctx context.Context, db *pgxpool.Pool, schema, table string) (int64, error) {
	idf := pgx.Identifier{schema, table}
	var n int64
	q := "SELECT COUNT(*) FROM " + idf.Sanitize()
	if err := db.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, dbutil.ErrWrap("table.count", err, dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("table", table))
	}
	return n, nil
}

// DeleteBackupsOlderThan deletes backups older than cutoff, respecting retention_until if set in the past.
func DeleteBackupsOlderThan(ctx context.Context, db *pgxpool.Pool, schema string, cutoff time.Time) (int64, error) {
	if schema == "" {
		schema = "backup"
	}
	ct, err := db.Exec(ctx, `DELETE FROM `+schema+`.backups WHERE created_at < $1 AND (retention_until IS NULL OR retention_until < now())`, cutoff)
	if err != nil {
		return 0, dbutil.ErrWrap("backup.prune", err, dbutil.ParamSummary("schema", schema))
	}
	return ct.RowsAffected(), nil
}

// CountBackupsOlderThan returns how many backups qualify for pruning at the cutoff, respecting retention_until if set (future or present prevents deletion).
func CountBackupsOlderThan(ctx context.Context, db *pgxpool.Pool, schema string, cutoff time.Time) (int64, error) {
	if schema == "" {
		schema = "backup"
	}
	var n int64
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM `+schema+`.backups WHERE created_at < $1 AND (retention_until IS NULL OR retention_until < now())`, cutoff).Scan(&n)
	if err != nil {
		return 0, dbutil.ErrWrap("backup.prune.count", err, dbutil.ParamSummary("schema", schema))
	}
	return n, nil
}

// Helper: small int to string without fmt to avoid allocations here.
func itoa(i int) string { return strconv.Itoa(i) }
