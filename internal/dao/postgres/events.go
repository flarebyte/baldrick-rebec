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
    ContentID      int64
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

// UpdateMessageEvent updates mutable fields of a message event: status, processed_at,
// error_message, content_id, executor, tags and meta. Any zero-value/empty inputs are ignored
// unless explicitly provided via sql.Null* with Valid=true.
func UpdateMessageEvent(ctx context.Context, db *pgxpool.Pool, id int64, update MessageEvent) error {
    // Build a compact UPDATE with COALESCE on provided fields while staying parameterized.
    // We purposely keep a fixed-shape query to avoid dynamic SQL per DB guidelines.
    metaJSON, _ := json.Marshal(update.Meta)
    q := `UPDATE messages SET
            status = COALESCE(NULLIF($1,''), status),
            processed_at = COALESCE($2, processed_at),
            error_message = COALESCE($3, error_message),
            content_id = COALESCE(NULLIF($4,0), content_id),
            executor = COALESCE($5, executor),
            tags = COALESCE($6::text[], tags),
            meta = COALESCE($7, meta)
          WHERE id=$8`
    _, err := db.Exec(ctx, q,
        update.Status,
        nullOrTime(update.ProcessedAt),
        nullOrString(update.ErrorMessage),
        update.ContentID,
        nullOrString(update.Executor),
        pgTextArrayOrNil(update.Tags),
        jsonOrNil(metaJSON),
        id,
    )
    return err
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

// ListMessages lists messages with optional filters.
func ListMessages(ctx context.Context, db *pgxpool.Pool, experimentID, taskID int64, status string, limit, offset int) ([]MessageEvent, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    // Build simple filter branches to stay parameterized and avoid dynamic SQL
    var rows pgxRows
    var err error
    switch {
    case experimentID > 0 && taskID > 0 && status != "":
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages WHERE experiment_id=$1 AND task_id=$2 AND status=$3
                                   ORDER BY received_at DESC LIMIT $4 OFFSET $5`, experimentID, taskID, status, limit, offset)
    case experimentID > 0 && taskID > 0:
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages WHERE experiment_id=$1 AND task_id=$2
                                   ORDER BY received_at DESC LIMIT $3 OFFSET $4`, experimentID, taskID, limit, offset)
    case experimentID > 0 && status != "":
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages WHERE experiment_id=$1 AND status=$2
                                   ORDER BY received_at DESC LIMIT $3 OFFSET $4`, experimentID, status, limit, offset)
    case taskID > 0 && status != "":
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages WHERE task_id=$1 AND status=$2
                                   ORDER BY received_at DESC LIMIT $3 OFFSET $4`, taskID, status, limit, offset)
    case experimentID > 0:
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages WHERE experiment_id=$1
                                   ORDER BY received_at DESC LIMIT $2 OFFSET $3`, experimentID, limit, offset)
    case taskID > 0:
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages WHERE task_id=$1
                                   ORDER BY received_at DESC LIMIT $2 OFFSET $3`, taskID, limit, offset)
    case status != "":
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages WHERE status=$1
                                   ORDER BY received_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
    default:
        rows, err = db.Query(ctx, `SELECT id, content_id, task_id, experiment_id, executor, received_at, processed_at, status, error_message, tags, meta
                                   FROM messages ORDER BY received_at DESC LIMIT $1 OFFSET $2`, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []MessageEvent
    for rows.Next() {
        var r MessageEvent
        var tags []string
        var metaBytes []byte
        if err := rows.Scan(&r.ID, &r.ContentID, &r.TaskID, &r.ExperimentID, &r.Executor, &r.ReceivedAt, &r.ProcessedAt, &r.Status, &r.ErrorMessage, &tags, &metaBytes); err != nil {
            return nil, err
        }
        r.Tags = tags
        if len(metaBytes) > 0 { _ = json.Unmarshal(metaBytes, &r.Meta) }
        out = append(out, r)
    }
    return out, rows.Err()
}

// DeleteMessage deletes a message by id.
func DeleteMessage(ctx context.Context, db *pgxpool.Pool, id int64) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM messages WHERE id=$1`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
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

// pgTextArrayOrNil returns nil when empty to avoid overriding with an empty array
// when the caller intended to leave the field unchanged.
func pgTextArrayOrNil(arr []string) any {
    if len(arr) == 0 { return nil }
    return arr
}

// jsonOrNil returns nil when the JSON payload is empty so the UPDATE can COALESCE.
func jsonOrNil(b []byte) any {
    if len(b) == 0 { return nil }
    // Accept "{}" or other content as-is.
    return b
}
