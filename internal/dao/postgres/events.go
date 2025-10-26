package postgres

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type MessageEvent struct {
    ID             int64
    ContentID      string
    TaskID         sql.NullInt64
    ExperimentID   sql.NullInt64
    Executor       sql.NullString
    ReceivedAt     time.Time
    ProcessedAt    sql.NullTime
    Status         string
    ErrorMessage   sql.NullString
    Tags           []string
    Meta           map[string]any
}

func InsertMessageEvent(ctx context.Context, db *pgxpool.Pool, ev *MessageEvent) (int64, error) {
    if ev == nil {
        return 0, errors.New("nil event")
    }
    metaJSON, _ := json.Marshal(ev.Meta)
    q := `INSERT INTO messages (
            content_id, task_id, experiment_id, executor,
            received_at, processed_at, status, error_message, tags, meta
        ) VALUES (
            $1,$2,$3,$4,
            COALESCE($5, now()),$6,$7,$8,$9::text[],$10
        ) RETURNING id`
    var id int64
    var receivedAt any
    if ev.ReceivedAt.IsZero() {
        receivedAt = nil
    } else {
        receivedAt = ev.ReceivedAt
    }
    err := db.QueryRow(ctx, q,
        ev.ContentID, nullOrInt64(ev.TaskID), nullOrInt64(ev.ExperimentID), nullOrString(ev.Executor),
        receivedAt, nullOrTime(ev.ProcessedAt), ev.Status, nullOrString(ev.ErrorMessage), ev.Tags, metaJSON,
    ).Scan(&id)
    if err != nil {
        return 0, err
    }
    ev.ID = id
    return id, nil
}

func GetMessageEventByID(ctx context.Context, db *pgxpool.Pool, id int64) (*MessageEvent, error) {
    q := `SELECT id, content_id,
                 task_id, experiment_id, executor,
                 received_at, processed_at, status, error_message, tags, meta
          FROM messages WHERE id=$1`
    row := db.QueryRow(ctx, q, id)
    var out MessageEvent
    var metaBytes []byte
    var tags []string
    var taskID, expID sql.NullInt64
    err := row.Scan(
        &out.ID, &out.ContentID,
        &taskID, &expID, &out.Executor,
        &out.ReceivedAt, &out.ProcessedAt, &out.Status, &out.ErrorMessage, &tags, &metaBytes,
    )
    if err != nil {
        return nil, err
    }
    out.TaskID = taskID
    out.ExperimentID = expID
    out.Tags = tags
    if len(metaBytes) > 0 {
        _ = json.Unmarshal(metaBytes, &out.Meta)
    }
    return &out, nil
}

func nullOrString(ns sql.NullString) any {
    if ns.Valid {
        return ns.String
    }
    return nil
}

func nullOrTime(nt sql.NullTime) any {
    if nt.Valid {
        return nt.Time
    }
    return nil
}

func stringOrEmpty(ns sql.NullString) string { if ns.Valid { return ns.String }; return "" }
func nullOrInt64(ni sql.NullInt64) any { if ni.Valid { return ni.Int64 }; return nil }
