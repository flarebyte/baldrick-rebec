package postgres

import (
    "context"
    "database/sql"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Experiment struct {
    ID             int64
    ConversationID int64
    Created        sql.NullTime
}

func CreateExperiment(ctx context.Context, db *pgxpool.Pool, conversationID int64) (*Experiment, error) {
    q := `INSERT INTO experiments (conversation_id) VALUES ($1) RETURNING id, created`
    var e Experiment
    e.ConversationID = conversationID
    if err := db.QueryRow(ctx, q, conversationID).Scan(&e.ID, &e.Created); err != nil {
        return nil, err
    }
    return &e, nil
}

func GetExperimentByID(ctx context.Context, db *pgxpool.Pool, id int64) (*Experiment, error) {
    q := `SELECT id, conversation_id, created FROM experiments WHERE id=$1`
    var e Experiment
    if err := db.QueryRow(ctx, q, id).Scan(&e.ID, &e.ConversationID, &e.Created); err != nil {
        return nil, err
    }
    return &e, nil
}

func ListExperiments(ctx context.Context, db *pgxpool.Pool, conversationID int64, limit, offset int) ([]Experiment, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    var rows pgxRows
    var err error
    if conversationID > 0 {
        rows, err = db.Query(ctx, `SELECT id, conversation_id, created FROM experiments WHERE conversation_id=$1 ORDER BY created DESC LIMIT $2 OFFSET $3`, conversationID, limit, offset)
    } else {
        rows, err = db.Query(ctx, `SELECT id, conversation_id, created FROM experiments ORDER BY created DESC LIMIT $1 OFFSET $2`, limit, offset)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Experiment
    for rows.Next() {
        var e Experiment
        if err := rows.Scan(&e.ID, &e.ConversationID, &e.Created); err != nil {
            return nil, err
        }
        out = append(out, e)
    }
    return out, rows.Err()
}

func DeleteExperiment(ctx context.Context, db *pgxpool.Pool, id int64) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM experiments WHERE id=$1`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

// helpers reused from other files
type pgxRows interface{ Next() bool; Scan(...any) error; Close(); Err() error }
