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
    ConversationID string
    AttemptID      string
    ProfileName    string
    Title          sql.NullString
    Level          sql.NullString
    SenderID       sql.NullString
    Recipients     []string
    Description    sql.NullString
    Goal           sql.NullString
    Timeout        sql.NullString // textual interval, e.g., '5 minutes'
    Source         string
    ReceivedAt     time.Time
    ProcessedAt    sql.NullTime
    Status         string
    ErrorMessage   sql.NullString
    Tags           []string
    Meta           map[string]any
    Attempt        int
}

func InsertMessageEvent(ctx context.Context, db *pgxpool.Pool, ev *MessageEvent) (int64, error) {
    if ev == nil {
        return 0, errors.New("nil event")
    }
    metaJSON, _ := json.Marshal(ev.Meta)
    // Note: cast timeout text to interval if provided
    q := `INSERT INTO messages_events (
            content_id, conversation_id, attempt_id, profile_name,
            title, level, sender_id, recipients, description, goal, timeout,
            source, received_at, processed_at, status, error_message, tags, meta, attempt
        ) VALUES (
            $1,$2,$3,$4,
            $5,$6,$7,$8::text[],$9,$10,CASE WHEN $11='' THEN NULL ELSE $11::interval END,
            $12,COALESCE($13, now()),$14,$15,$16,$17::text[],$18,$19
        ) RETURNING id`
    var id int64
    var receivedAt any
    if ev.ReceivedAt.IsZero() {
        receivedAt = nil
    } else {
        receivedAt = ev.ReceivedAt
    }
    err := db.QueryRow(ctx, q,
        ev.ContentID, ev.ConversationID, ev.AttemptID, ev.ProfileName,
        nullOrString(ev.Title), nullOrString(ev.Level), nullOrString(ev.SenderID), ev.Recipients,
        nullOrString(ev.Description), nullOrString(ev.Goal), stringOrEmpty(ev.Timeout),
        ev.Source, receivedAt, nullOrTime(ev.ProcessedAt), ev.Status, nullOrString(ev.ErrorMessage), ev.Tags, metaJSON, ev.Attempt,
    ).Scan(&id)
    if err != nil {
        return 0, err
    }
    ev.ID = id
    return id, nil
}

func GetMessageEventByID(ctx context.Context, db *pgxpool.Pool, id int64) (*MessageEvent, error) {
    q := `SELECT id, content_id, conversation_id, attempt_id, profile_name,
                 title, level, sender_id, recipients, description, goal, timeout,
                 source, received_at, processed_at, status, error_message, tags, meta, attempt
          FROM messages_events WHERE id=$1`
    row := db.QueryRow(ctx, q, id)
    var out MessageEvent
    var metaBytes []byte
    var timeout sql.NullString
    var recipients, tags []string
    err := row.Scan(
        &out.ID, &out.ContentID, &out.ConversationID, &out.AttemptID, &out.ProfileName,
        &out.Title, &out.Level, &out.SenderID, &recipients, &out.Description, &out.Goal, &timeout,
        &out.Source, &out.ReceivedAt, &out.ProcessedAt, &out.Status, &out.ErrorMessage, &tags, &metaBytes, &out.Attempt,
    )
    if err != nil {
        return nil, err
    }
    out.Timeout = timeout
    out.Recipients = recipients
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

func stringOrEmpty(ns sql.NullString) string {
    if ns.Valid {
        return ns.String
    }
    return ""
}
