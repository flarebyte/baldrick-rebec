package postgres

import (
    "context"
    "database/sql"

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

