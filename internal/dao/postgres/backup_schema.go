package postgres

import (
	"context"
	"fmt"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureBackupSchema creates the dedicated backup schema and tables if missing.
// It does not require superuser privileges.
func EnsureBackupSchema(ctx context.Context, db *pgxpool.Pool, schema string) error {
	if schema == "" {
		schema = "backup"
	}
	sid := pgx.Identifier{schema}.Sanitize()
	qual := func(tbl string) string { return pgx.Identifier{schema, tbl}.Sanitize() }
	stmts := []string{
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, sid),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            description TEXT NULL,
            tags JSONB NOT NULL DEFAULT '{}'::jsonb,
            initiated_by TEXT NULL,
            retention_until TIMESTAMPTZ NULL,
            full_schema JSONB NULL
        )`, qual("backups")),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            backup_id UUID NOT NULL REFERENCES %s ON DELETE CASCADE,
            entity_name TEXT NOT NULL,
            field_name TEXT NOT NULL,
            field_type TEXT NOT NULL,
            is_nullable BOOLEAN NOT NULL,
            default_value TEXT NULL,
            metadata JSONB NULL,
            PRIMARY KEY (backup_id, entity_name, field_name)
        )`, qual("entity_schema"), qual("backups")),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            id BIGSERIAL PRIMARY KEY,
            backup_id UUID NOT NULL REFERENCES %s ON DELETE CASCADE,
            entity_name TEXT NOT NULL,
            record_pk JSONB NOT NULL,
            record JSONB NOT NULL,
            role_name TEXT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`, qual("entity_records"), qual("backups")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_entity_records_bk_entity ON %s(backup_id, entity_name)`, qual("entity_records")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_entity_records_pk ON %s(backup_id, entity_name, record_pk)`, qual("entity_records")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_entity_records_role ON %s(role_name)`, qual("entity_records")),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            id BIGSERIAL PRIMARY KEY,
            backup_id UUID NOT NULL REFERENCES %s ON DELETE CASCADE,
            entity_name TEXT NOT NULL,
            record_pk JSONB NOT NULL,
            field_name TEXT NOT NULL,
            field_type TEXT NOT NULL,
            role_name TEXT NULL,
            is_override BOOLEAN NOT NULL DEFAULT true,
            value_text TEXT NULL,
            value_number NUMERIC NULL,
            value_bool BOOLEAN NULL,
            value_timestamp TIMESTAMPTZ NULL,
            value_jsonb JSONB NULL
        )`, qual("entity_fields"), qual("backups")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_entity_fields_bk_entity ON %s(backup_id, entity_name)`, qual("entity_fields")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_entity_fields_rec_field ON %s(record_pk, field_name)`, qual("entity_fields")),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_entity_fields_role ON %s(role_name)`, qual("entity_fields")),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            backup_id UUID NOT NULL REFERENCES %s ON DELETE CASCADE,
            name TEXT NOT NULL,
            description TEXT NULL,
            script TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            applied BOOLEAN NOT NULL DEFAULT false,
            applied_at TIMESTAMPTZ NULL
        )`, qual("migrations"), qual("backups")),
	}
	for i, s := range stmts {
		if _, err := db.Exec(ctx, s); err != nil {
			return dbutil.ErrWrap("backup.ensure_schema", err,
				dbutil.ParamSummary("schema", schema),
				fmt.Sprintf("stmt_index=%d", i))
		}
	}
	return nil
}

// EnsureBackupSchemaGrants ensures the backup schema exists and grants privileges to the given role.
// It sets the schema owner to the role if schema is newly created (or already exists, GRANTs still applied).
func EnsureBackupSchemaGrants(ctx context.Context, db *pgxpool.Pool, schema, role string) error {
	if schema == "" {
		schema = "backup"
	}
	sid := pgx.Identifier{schema}.Sanitize()
	rid := pgx.Identifier{role}.Sanitize()
	qual := func(tbl string) string { return pgx.Identifier{schema, tbl}.Sanitize() }
	stmts := []string{
		// Create schema owned by role; IF NOT EXISTS avoids errors if present
		fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s AUTHORIZATION %s", sid, rid),
		// Basic usage
		fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", sid, rid),
		// Allow creating tables/sequences in the backup schema
		fmt.Sprintf("GRANT CREATE ON SCHEMA %s TO %s", sid, rid),
		// Table privileges (existing and future)
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %s TO %s", sid, rid),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s", sid, rid),
		// Sequence privileges for BIGSERIAL, etc.
		fmt.Sprintf("GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA %s TO %s", sid, rid),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT USAGE, SELECT ON SEQUENCES TO %s", sid, rid),
		// Ensure ownership is consistent so backup role can manage indexes
		fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", sid, rid),
		fmt.Sprintf("ALTER TABLE IF EXISTS %s OWNER TO %s", qual("backups"), rid),
		fmt.Sprintf("ALTER TABLE IF EXISTS %s OWNER TO %s", qual("entity_schema"), rid),
		fmt.Sprintf("ALTER TABLE IF EXISTS %s OWNER TO %s", qual("entity_records"), rid),
		fmt.Sprintf("ALTER TABLE IF EXISTS %s OWNER TO %s", qual("entity_fields"), rid),
		fmt.Sprintf("ALTER TABLE IF EXISTS %s OWNER TO %s", qual("migrations"), rid),
		fmt.Sprintf("ALTER SEQUENCE IF EXISTS %s OWNER TO %s", pgx.Identifier{schema, "entity_records_id_seq"}.Sanitize(), rid),
		fmt.Sprintf("ALTER SEQUENCE IF EXISTS %s OWNER TO %s", pgx.Identifier{schema, "entity_fields_id_seq"}.Sanitize(), rid),
	}
	for i, s := range stmts {
		if _, err := db.Exec(ctx, s); err != nil {
			return dbutil.ErrWrap("backup.ensure_schema_grants", err,
				dbutil.ParamSummary("schema", schema), dbutil.ParamSummary("role", role),
				fmt.Sprintf("stmt_index=%d", i))
		}
	}
	return nil
}
