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
    host := cfg.Postgres.Host
    port := cfg.Postgres.Port
    user := cfg.Postgres.User
    pass := cfg.Postgres.Password
    dbname := cfg.Postgres.DBName
    sslmode := cfg.Postgres.SSLMode
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", host, port, user, pass, dbname, sslmode)
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

