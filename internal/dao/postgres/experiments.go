package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
)

type Experiment struct {
    ID             string
    ConversationID string
    Created        string // RFC3339 timestamp as string for simplicity
}

func CreateExperiment(ctx context.Context, db *pgxpool.Pool, conversationID string) (*Experiment, error) {
    q := `INSERT INTO experiments (conversation_id) VALUES ($1::uuid) RETURNING id::text, created`
    var e Experiment
    e.ConversationID = conversationID
    var createdTS any
    if err := db.QueryRow(ctx, q, conversationID).Scan(&e.ID, &createdTS); err != nil {
        return nil, dbutil.ErrWrap("experiment.insert", err, dbutil.ParamSummary("conversation_id", conversationID))
    }
    // pgx encodes time as time.Time; format to RFC3339
    switch t := createdTS.(type) {
    case string:
        e.Created = t
    default:
        // Best effort
        // let JSON marshalling handle types elsewhere if needed
    }
    return &e, nil
}

func GetExperimentByID(ctx context.Context, db *pgxpool.Pool, id string) (*Experiment, error) {
    q := `SELECT id::text, conversation_id::text, created FROM experiments WHERE id=$1::uuid`
    var e Experiment
    var createdTS any
    if err := db.QueryRow(ctx, q, id).Scan(&e.ID, &e.ConversationID, &createdTS); err != nil {
        return nil, dbutil.ErrWrap("experiment.get", err, dbutil.ParamSummary("id", id))
    }
    switch t := createdTS.(type) {
    case string:
        e.Created = t
    }
    return &e, nil
}

func ListExperiments(ctx context.Context, db *pgxpool.Pool, conversationID string, limit, offset int) ([]Experiment, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    if stringsTrim(conversationID) != "" {
        rows, err = db.Query(ctx, `SELECT id::text, conversation_id::text, created FROM experiments WHERE conversation_id=$1::uuid ORDER BY created DESC LIMIT $2 OFFSET $3`, conversationID, limit, offset)
    } else {
        rows, err = db.Query(ctx, `SELECT id::text, conversation_id::text, created FROM experiments ORDER BY created DESC LIMIT $1 OFFSET $2`, limit, offset)
    }
    if err != nil { return nil, dbutil.ErrWrap("experiment.list", err, dbutil.ParamSummary("conversation_id", conversationID), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset)) }
    defer rows.Close()
    var out []Experiment
    for rows.Next() {
        var e Experiment
        var createdTS any
        if err := rows.Scan(&e.ID, &e.ConversationID, &createdTS); err != nil {
            return nil, dbutil.ErrWrap("experiment.list.scan", err)
        }
        if s, ok := createdTS.(string); ok { e.Created = s }
        out = append(out, e)
    }
    if err := rows.Err(); err != nil { return nil, dbutil.ErrWrap("experiment.list", err) }
    return out, nil
}

func DeleteExperiment(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM experiments WHERE id=$1::uuid`, id)
    if err != nil { return 0, dbutil.ErrWrap("experiment.delete", err, dbutil.ParamSummary("id", id)) }
    return ct.RowsAffected(), nil
}
