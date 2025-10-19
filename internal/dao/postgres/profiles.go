package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type MessageProfile struct {
    ID          int64
    Name        string
    Description sql.NullString
    Goal        sql.NullString
    Tags        []string
    IsVector    bool
    Timeout     sql.NullString // textual interval
    Sensitive   bool
    Title       sql.NullString
    Level       sql.NullString
    SenderID    sql.NullString
    CreatedAt   sql.NullTime
    UpdatedAt   sql.NullTime
}

func UpsertMessageProfile(ctx context.Context, db *pgxpool.Pool, p *MessageProfile) (int64, error) {
    // Use INSERT ... ON CONFLICT (name) DO UPDATE
    q := `INSERT INTO message_profiles (
            name, description, goal, tags, is_vector, timeout, sensitive,
            title, level, sender_id
        ) VALUES (
            $1,$2,$3,$4::text[],$5,CASE WHEN $6='' THEN NULL ELSE $6::interval END,$7,
            $8,$9,$10
        )
        ON CONFLICT (name) DO UPDATE SET
            description = EXCLUDED.description,
            goal = EXCLUDED.goal,
            tags = EXCLUDED.tags,
            is_vector = EXCLUDED.is_vector,
            timeout = EXCLUDED.timeout,
            sensitive = EXCLUDED.sensitive,
            title = EXCLUDED.title,
            level = EXCLUDED.level,
            sender_id = EXCLUDED.sender_id,
            updated_at = now()
        RETURNING id`
    var id int64
    if err := db.QueryRow(ctx, q,
        p.Name, nullOrString(p.Description), nullOrString(p.Goal), p.Tags, p.IsVector, stringOrEmpty(p.Timeout), p.Sensitive,
        nullOrString(p.Title), nullOrString(p.Level), nullOrString(p.SenderID),
    ).Scan(&id); err != nil {
        return 0, err
    }
    p.ID = id
    return id, nil
}

func GetMessageProfileByName(ctx context.Context, db *pgxpool.Pool, name string) (*MessageProfile, error) {
    q := `SELECT id, name, description, goal, tags, is_vector, timeout, sensitive,
                 title, level, sender_id, created_at, updated_at
          FROM message_profiles WHERE name=$1`
    row := db.QueryRow(ctx, q, name)
    var out MessageProfile
    var timeout sql.NullString
    var tags []string
    if err := row.Scan(
        &out.ID, &out.Name, &out.Description, &out.Goal, &tags, &out.IsVector, &timeout, &out.Sensitive,
        &out.Title, &out.Level, &out.SenderID, &out.CreatedAt, &out.UpdatedAt,
    ); err != nil {
        return nil, err
    }
    out.Tags = tags
    out.Timeout = timeout
    return &out, nil
}
