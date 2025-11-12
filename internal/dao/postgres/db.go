package postgres

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
    "github.com/jackc/pgx/v5/pgxpool"
)

// Open returns a pgxpool.Pool with sane defaults using the provided config.
func Open(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
    // Prefer app role; fallback to admin
    user := cfg.Postgres.App.User
    pass := cfg.Postgres.App.Password
    if user == "" {
        user = cfg.Postgres.Admin.User
        pass = cfg.Postgres.Admin.Password
    }
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
    return openPool(ctx, dsn)
}

// OpenApp opens using app role credentials.
func OpenApp(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
    user := cfg.Postgres.App.User
    pass := cfg.Postgres.App.Password
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
    return openPool(ctx, dsn)
}

// OpenAdmin opens using admin role credentials.
func OpenAdmin(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
    user := cfg.Postgres.Admin.User
    pass := cfg.Postgres.Admin.Password
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
    return openPool(ctx, dsn)
}

// OpenAdminWithDB opens using admin role to a specific database name.
func OpenAdminWithDB(ctx context.Context, cfg config.Config, dbName string) (*pgxpool.Pool, error) {
    user := cfg.Postgres.Admin.User
    pass := cfg.Postgres.Admin.Password
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, user, pass, dbName, cfg.Postgres.SSLMode)
    return openPool(ctx, dsn)
}

func openPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    // Pool sizing — tune as needed
    cfg.MaxConns = 10
    cfg.MinConns = 1
    cfg.MaxConnLifetime = 30 * time.Minute
    cfg.MaxConnIdleTime = 5 * time.Minute

    // Session bootstrap: set a sane search_path for application schemas
    cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
        _, _ = conn.Exec(ctx, `SET search_path = "$user", public`)
        return nil
    }

    // Create pool
    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, err
    }
    // Retry ping with short exponential backoff. Fail fast on auth errors.
    var lastErr error
    attempts := 3
    for i := 0; i < attempts; i++ {
        // Short per-attempt timeout
        pingCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
        lastErr = pool.Ping(pingCtx)
        cancel()
        if lastErr == nil {
            return pool, nil
        }
        var pgErr *pgconn.PgError
        if errors.As(lastErr, &pgErr) {
            // Authentication or authorization error: do not retry.
            if pgErr.Code == "28P01" || pgErr.Code == "28000" {
                break
            }
        }
        time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
    }
    pool.Close()
    return nil, lastErr
}

func firstNonEmpty(values ...string) string {
    for _, v := range values {
        if v != "" {
            return v
        }
    }
    return ""
}
