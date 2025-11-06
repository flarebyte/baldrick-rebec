package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Stickie struct {
    ID              string
    BlackboardID    string
    TopicName       sql.NullString
    TopicRoleName   sql.NullString
    Note            sql.NullString
    Labels          []string
    Created         sql.NullTime
    Updated         sql.NullTime
    CreatedByTaskID sql.NullString
    EditCount       int
    PriorityLevel   sql.NullString
}

// UpsertStickie inserts a new stickie if ID is empty, otherwise updates it.
func UpsertStickie(ctx context.Context, db *pgxpool.Pool, s *Stickie) error {
    if s.ID != "" {
        q := `UPDATE stickies
              SET blackboard_id=$2::uuid,
                  topic_name=NULLIF($3,''),
                  topic_role_name=NULLIF($4,''),
                  note=NULLIF($5,''),
                  labels=COALESCE($6, labels),
                  created_by_task_id=CASE WHEN $7='' THEN NULL ELSE $7::uuid END,
                  priority_level=NULLIF($8,'')
              WHERE id=$1::uuid
              RETURNING created, updated, edit_count`
        return db.QueryRow(ctx, q,
            s.ID, s.BlackboardID, stringOrEmpty(s.TopicName), stringOrEmpty(s.TopicRoleName), stringOrEmpty(s.Note),
            pgTextArrayOrNil(s.Labels), stringOrEmpty(s.CreatedByTaskID), stringOrEmpty(s.PriorityLevel),
        ).Scan(&s.Created, &s.Updated, &s.EditCount)
    }
    q := `INSERT INTO stickies (blackboard_id, topic_name, topic_role_name, note, labels, created_by_task_id, priority_level)
          VALUES ($1::uuid, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), COALESCE($5,ARRAY[]::text[]), CASE WHEN $6='' THEN NULL ELSE $6::uuid END, NULLIF($7,''))
          RETURNING id::text, created, updated, edit_count`
    return db.QueryRow(ctx, q,
        s.BlackboardID, stringOrEmpty(s.TopicName), stringOrEmpty(s.TopicRoleName), stringOrEmpty(s.Note), pgTextArrayOrNil(s.Labels), stringOrEmpty(s.CreatedByTaskID), stringOrEmpty(s.PriorityLevel),
    ).Scan(&s.ID, &s.Created, &s.Updated, &s.EditCount)
}

// GetStickieByID fetches a stickie by UUID.
func GetStickieByID(ctx context.Context, db *pgxpool.Pool, id string) (*Stickie, error) {
    q := `SELECT id::text, blackboard_id::text, topic_name, topic_role_name, note, labels, created, updated, created_by_task_id::text, edit_count, priority_level
          FROM stickies WHERE id=$1::uuid`
    var s Stickie
    if err := db.QueryRow(ctx, q, id).Scan(&s.ID, &s.BlackboardID, &s.TopicName, &s.TopicRoleName, &s.Note, &s.Labels, &s.Created, &s.Updated, &s.CreatedByTaskID, &s.EditCount, &s.PriorityLevel); err != nil {
        return nil, err
    }
    return &s, nil
}

// ListStickies lists stickies with optional filters.
func ListStickies(ctx context.Context, db *pgxpool.Pool, blackboardID, topicName, topicRole string, limit, offset int) ([]Stickie, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    switch {
    case stringsTrim(blackboardID) != "" && stringsTrim(topicName) != "" && stringsTrim(topicRole) != "":
        rows, err = db.Query(ctx, `SELECT id::text, blackboard_id::text, topic_name, topic_role_name, note, labels, created, updated, created_by_task_id::text, edit_count, priority_level
                                   FROM stickies WHERE blackboard_id=$1::uuid AND topic_name=$2 AND topic_role_name=$3
                                   ORDER BY updated DESC, created DESC LIMIT $4 OFFSET $5`, blackboardID, topicName, topicRole, limit, offset)
    case stringsTrim(blackboardID) != "":
        rows, err = db.Query(ctx, `SELECT id::text, blackboard_id::text, topic_name, topic_role_name, note, labels, created, updated, created_by_task_id::text, edit_count, priority_level
                                   FROM stickies WHERE blackboard_id=$1::uuid
                                   ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`, blackboardID, limit, offset)
    case stringsTrim(topicName) != "" && stringsTrim(topicRole) != "":
        rows, err = db.Query(ctx, `SELECT id::text, blackboard_id::text, topic_name, topic_role_name, note, labels, created, updated, created_by_task_id::text, edit_count, priority_level
                                   FROM stickies WHERE topic_name=$1 AND topic_role_name=$2
                                   ORDER BY updated DESC, created DESC LIMIT $3 OFFSET $4`, topicName, topicRole, limit, offset)
    default:
        rows, err = db.Query(ctx, `SELECT id::text, blackboard_id::text, topic_name, topic_role_name, note, labels, created, updated, created_by_task_id::text, edit_count, priority_level
                                   FROM stickies ORDER BY updated DESC, created DESC LIMIT $1 OFFSET $2`, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Stickie
    for rows.Next() {
        var s Stickie
        if err := rows.Scan(&s.ID, &s.BlackboardID, &s.TopicName, &s.TopicRoleName, &s.Note, &s.Labels, &s.Created, &s.Updated, &s.CreatedByTaskID, &s.EditCount, &s.PriorityLevel); err != nil {
            return nil, err
        }
        out = append(out, s)
    }
    return out, rows.Err()
}

// DeleteStickie deletes by id.
func DeleteStickie(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM stickies WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

