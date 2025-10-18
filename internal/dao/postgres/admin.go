package postgres

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "regexp"
    pqpkg "github.com/lib/pq"
)

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func safeIdent(name string) (string, error) {
    if !identRe.MatchString(name) {
        return "", fmt.Errorf("invalid identifier: %q", name)
    }
    return name, nil
}

// EnsureRole creates a role with LOGIN if it doesn't exist and sets password if provided.
func EnsureRole(ctx context.Context, db *sql.DB, roleName, password string) error {
    if roleName == "" {
        return errors.New("empty role name")
    }
    rn, err := safeIdent(roleName)
    if err != nil { return err }
    // Create role if missing
    _, err = db.ExecContext(ctx, fmt.Sprintf(
        "DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN CREATE ROLE %s LOGIN; END IF; END $$;",
        rn, rn,
    ))
    if err != nil { return err }
    // Set password if provided
    if password != "" {
        // Use literal quoting to avoid placeholder issues in ALTER ROLE
        quoted := pqpkg.QuoteLiteral(password)
        stmt := fmt.Sprintf("ALTER ROLE %s WITH LOGIN PASSWORD %s", rn, quoted)
        if _, err := db.ExecContext(ctx, stmt); err != nil { return err }
    }
    return nil
}

// EnsureDatabase creates a database if it doesn't exist, with an optional owner.
func EnsureDatabase(ctx context.Context, db *sql.DB, dbName, owner string) error {
    if dbName == "" { return errors.New("empty database name") }
    dn, err := safeIdent(dbName)
    if err != nil { return err }
    ow := owner
    if ow != "" {
        if ow, err = safeIdent(ow); err != nil { return err }
    }
    // Check existence
    var exists bool
    if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", dn).Scan(&exists); err != nil {
        return err
    }
    if exists { return nil }
    // Create
    stmt := fmt.Sprintf("CREATE DATABASE %s", dn)
    if ow != "" { stmt += fmt.Sprintf(" OWNER %s", ow) }
    _, err = db.ExecContext(ctx, stmt)
    return err
}

// GrantConnect grants CONNECT on database to app role.
func GrantConnect(ctx context.Context, db *sql.DB, dbName, appRole string) error {
    if dbName == "" || appRole == "" { return errors.New("empty db or role") }
    dn, err := safeIdent(dbName)
    if err != nil { return err }
    ar, err := safeIdent(appRole)
    if err != nil { return err }
    _, err = db.ExecContext(ctx, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", dn, ar))
    return err
}

// GrantRuntimePrivileges grants typical privileges for app role inside the connected database.
func GrantRuntimePrivileges(ctx context.Context, db *sql.DB, appRole string) error {
    if appRole == "" { return errors.New("empty app role") }
    ar, err := safeIdent(appRole)
    if err != nil { return err }
    stmts := []string{
        fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", ar),
        fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s", ar),
        fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s", ar),
        fmt.Sprintf("GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO %s", ar),
        fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO %s", ar),
    }
    for _, s := range stmts {
        if _, err := db.ExecContext(ctx, s); err != nil {
            return err
        }
    }
    return nil
}

// RevokeRuntimePrivileges revokes typical runtime privileges for the app role in the connected DB.
func RevokeRuntimePrivileges(ctx context.Context, db *sql.DB, appRole string) error {
    if appRole == "" { return errors.New("empty app role") }
    ar, err := safeIdent(appRole)
    if err != nil { return err }
    stmts := []string{
        fmt.Sprintf("REVOKE SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public FROM %s", ar),
        fmt.Sprintf("REVOKE USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public FROM %s", ar),
        fmt.Sprintf("REVOKE USAGE ON SCHEMA public FROM %s", ar),
        fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM %s", ar),
        fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE USAGE, SELECT ON SEQUENCES FROM %s", ar),
    }
    for _, s := range stmts {
        if _, err := db.ExecContext(ctx, s); err != nil {
            return err
        }
    }
    return nil
}

// RoleExists checks if a role exists.
func RoleExists(ctx context.Context, db *sql.DB, roleName string) (bool, error) {
    if roleName == "" { return false, errors.New("empty role name") }
    var ok bool
    err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", roleName).Scan(&ok)
    return ok, err
}

// DatabaseExists checks if a database exists.
func DatabaseExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
    if name == "" { return false, errors.New("empty db name") }
    var ok bool
    err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", name).Scan(&ok)
    return ok, err
}

// HasSchemaUsage returns true if role has USAGE on schema.
func HasSchemaUsage(ctx context.Context, db *sql.DB, role, schema string) (bool, error) {
    if role == "" || schema == "" { return false, errors.New("empty role or schema") }
    var ok bool
    err := db.QueryRowContext(ctx, "SELECT has_schema_privilege($1, $2, 'USAGE')", role, schema).Scan(&ok)
    return ok, err
}

// MissingTableDML returns true if any table in schema lacks DML privileges for role.
func MissingTableDML(ctx context.Context, db *sql.DB, role, schema string) (bool, error) {
    if role == "" || schema == "" { return false, errors.New("empty role or schema") }
    // Check for any table where role lacks at least one of SELECT/INSERT/UPDATE/DELETE
    q := `SELECT EXISTS (
            SELECT 1
            FROM information_schema.tables t
            WHERE t.table_schema=$1 AND t.table_type='BASE TABLE'
              AND (
                NOT has_table_privilege($2, quote_ident(t.table_schema)||'.'||quote_ident(t.table_name), 'SELECT') OR
                NOT has_table_privilege($2, quote_ident(t.table_schema)||'.'||quote_ident(t.table_name), 'INSERT') OR
                NOT has_table_privilege($2, quote_ident(t.table_schema)||'.'||quote_ident(t.table_name), 'UPDATE') OR
                NOT has_table_privilege($2, quote_ident(t.table_schema)||'.'||quote_ident(t.table_name), 'DELETE')
              )
          )`
    var anyMissing bool
    if err := db.QueryRowContext(ctx, q, schema, role).Scan(&anyMissing); err != nil {
        return false, err
    }
    return anyMissing, nil
}
