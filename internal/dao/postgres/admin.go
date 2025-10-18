package postgres

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "regexp"
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
        _, err = db.ExecContext(ctx, fmt.Sprintf("ALTER ROLE %s WITH LOGIN PASSWORD $1", rn), password)
        if err != nil { return err }
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

