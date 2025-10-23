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
    SenderID       sql.NullString
    Recipients     []string
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
    q := `INSERT INTO messages_events (
            content_id, conversation_id, attempt_id,
            sender_id, recipients,
            source, received_at, processed_at, status, error_message, tags, meta, attempt
        ) VALUES (
            $1,$2,$3,
            $4,$5::text[],
            $6,COALESCE($7, now()),$8,$9,$10,$11::text[],$12,$13
        ) RETURNING id`
    var id int64
    var receivedAt any
    if ev.ReceivedAt.IsZero() {
        receivedAt = nil
    } else {
        receivedAt = ev.ReceivedAt
    }
    err := db.QueryRow(ctx, q,
        ev.ContentID, ev.ConversationID, ev.AttemptID,
        nullOrString(ev.SenderID), ev.Recipients,
        ev.Source, receivedAt, nullOrTime(ev.ProcessedAt), ev.Status, nullOrString(ev.ErrorMessage), ev.Tags, metaJSON, ev.Attempt,
    ).Scan(&id)
    if err != nil {
        return 0, err
    }
    ev.ID = id
    return id, nil
}

func GetMessageEventByID(ctx context.Context, db *pgxpool.Pool, id int64) (*MessageEvent, error) {
    q := `SELECT id, content_id, conversation_id, attempt_id,
                 sender_id, recipients,
                 source, received_at, processed_at, status, error_message, tags, meta, attempt
          FROM messages_events WHERE id=$1`
    row := db.QueryRow(ctx, q, id)
    var out MessageEvent
    var metaBytes []byte
    var recipients, tags []string
    err := row.Scan(
        &out.ID, &out.ContentID, &out.ConversationID, &out.AttemptID,
        &out.SenderID, &recipients,
        &out.Source, &out.ReceivedAt, &out.ProcessedAt, &out.Status, &out.ErrorMessage, &tags, &metaBytes, &out.Attempt,
    )
    if err != nil {
        return nil, err
    }
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

func stringOrEmpty(ns sql.NullString) string { if ns.Valid { return ns.String }; return "" }
