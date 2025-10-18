package postgres

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    "github.com/flarebyte/baldrick-rebec/internal/config"
    _ "github.com/lib/pq"
)

// Open returns a sql.DB with sane defaults using the provided config.
func Open(ctx context.Context, cfg config.Config) (*sql.DB, error) {
    // Prefer app role; fallback to admin; then legacy
    user := cfg.Postgres.App.User
    pass := cfg.Postgres.App.Password
    if user == "" {
        user = cfg.Postgres.Admin.User
        if pass == "" {
            pass = firstNonEmpty(cfg.Postgres.Admin.Password, cfg.Postgres.Admin.PasswordTemp)
        }
    }
    if user == "" {
        user = cfg.Postgres.User
        pass = cfg.Postgres.Password
    }
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
    return openDSN(ctx, dsn)
}

// OpenApp opens using app role credentials (or legacy fallback).
func OpenApp(ctx context.Context, cfg config.Config) (*sql.DB, error) {
    user := cfg.Postgres.App.User
    pass := cfg.Postgres.App.Password
    if user == "" {
        user = cfg.Postgres.User
        pass = cfg.Postgres.Password
    }
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
    return openDSN(ctx, dsn)
}

// OpenAdmin opens using admin role credentials (prefers password_temp).
func OpenAdmin(ctx context.Context, cfg config.Config) (*sql.DB, error) {
    user := cfg.Postgres.Admin.User
    pass := firstNonEmpty(cfg.Postgres.Admin.Password, cfg.Postgres.Admin.PasswordTemp)
    if user == "" {
        // fallback to app, then legacy
        user = cfg.Postgres.App.User
        pass = cfg.Postgres.App.Password
        if user == "" {
            user = cfg.Postgres.User
            pass = cfg.Postgres.Password
        }
    }
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
    return openDSN(ctx, dsn)
}

// OpenAdminWithDB opens using admin role to a specific database name.
func OpenAdminWithDB(ctx context.Context, cfg config.Config, dbName string) (*sql.DB, error) {
    user := cfg.Postgres.Admin.User
    pass := firstNonEmpty(cfg.Postgres.Admin.Password, cfg.Postgres.Admin.PasswordTemp)
    if user == "" {
        // fallback to app, then legacy
        user = cfg.Postgres.App.User
        pass = cfg.Postgres.App.Password
        if user == "" {
            user = cfg.Postgres.User
            pass = cfg.Postgres.Password
        }
    }
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, dbName, cfg.Postgres.SSLMode)
    return openDSN(ctx, dsn)
}

func openDSN(ctx context.Context, dsn string) (*sql.DB, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(30 * time.Minute)
    // Ping to validate connectivity
    ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    if err := db.PingContext(ctxPing); err != nil {
        _ = db.Close()
        return nil, err
    }
    return db, nil
}

func firstNonEmpty(values ...string) string {
    for _, v := range values {
        if v != "" {
            return v
        }
    }
    return ""
}
