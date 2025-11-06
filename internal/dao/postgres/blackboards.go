package postgres

import (
    "context"
    "database/sql"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Blackboard struct {
    ID            string
    StoreID       string
    RoleName      string
    ConversationID sql.NullString
    ProjectName    sql.NullString
    TaskID         sql.NullString
    Background     sql.NullString
    Guidelines     sql.NullString
    Created        sql.NullTime
    Updated        sql.NullTime
}

// UpsertBlackboard inserts a new blackboard if ID is empty, otherwise updates it.
func UpsertBlackboard(ctx context.Context, db *pgxpool.Pool, b *Blackboard) error {
    if b.ID != "" {
        q := `UPDATE blackboards
              SET store_id=$2::uuid,
                  role_name=$3,
                  conversation_id=CASE WHEN $4='' THEN NULL ELSE $4::uuid END,
                  project_name=NULLIF($5,''),
                  task_id=CASE WHEN $6='' THEN NULL ELSE $6::uuid END,
                  background=NULLIF($7,''),
                  guidelines=NULLIF($8,''),
                  updated=now()
              WHERE id=$1::uuid
              RETURNING created, updated`
        return db.QueryRow(ctx, q,
            b.ID, b.StoreID, b.RoleName,
            stringOrEmpty(b.ConversationID), stringOrEmpty(b.ProjectName), stringOrEmpty(b.TaskID),
            stringOrEmpty(b.Background), stringOrEmpty(b.Guidelines),
        ).Scan(&b.Created, &b.Updated)
    }
    q := `INSERT INTO blackboards (store_id, role_name, conversation_id, project_name, task_id, background, guidelines)
          VALUES ($1::uuid, $2, CASE WHEN $3='' THEN NULL ELSE $3::uuid END, NULLIF($4,''), CASE WHEN $5='' THEN NULL ELSE $5::uuid END, NULLIF($6,''), NULLIF($7,''))
          RETURNING id::text, created, updated`
    return db.QueryRow(ctx, q,
        b.StoreID, b.RoleName, stringOrEmpty(b.ConversationID), stringOrEmpty(b.ProjectName), stringOrEmpty(b.TaskID), stringOrEmpty(b.Background), stringOrEmpty(b.Guidelines),
    ).Scan(&b.ID, &b.Created, &b.Updated)
}

// GetBlackboardByID fetches a blackboard by UUID.
func GetBlackboardByID(ctx context.Context, db *pgxpool.Pool, id string) (*Blackboard, error) {
    q := `SELECT id::text, store_id::text, role_name, conversation_id::text, project_name, task_id::text, background, guidelines, created, updated
          FROM blackboards WHERE id=$1::uuid`
    var b Blackboard
    if err := db.QueryRow(ctx, q, id).Scan(&b.ID, &b.StoreID, &b.RoleName, &b.ConversationID, &b.ProjectName, &b.TaskID, &b.Background, &b.Guidelines, &b.Created, &b.Updated); err != nil {
        return nil, err
    }
    return &b, nil
}

// ListBlackboards lists blackboards filtered by role.
func ListBlackboards(ctx context.Context, db *pgxpool.Pool, roleName string, limit, offset int) ([]Blackboard, error) {
    if limit <= 0 { limit = 100 }
    if offset < 0 { offset = 0 }
    q := `SELECT id::text, store_id::text, role_name, conversation_id::text, project_name, task_id::text, background, guidelines, created, updated
          FROM blackboards WHERE role_name=$1 ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`
    rows, err := db.Query(ctx, q, roleName, limit, offset)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Blackboard
    for rows.Next() {
        var b Blackboard
        if err := rows.Scan(&b.ID, &b.StoreID, &b.RoleName, &b.ConversationID, &b.ProjectName, &b.TaskID, &b.Background, &b.Guidelines, &b.Created, &b.Updated); err != nil {
            return nil, err
        }
        out = append(out, b)
    }
    return out, rows.Err()
}

// DeleteBlackboard deletes by id.
func DeleteBlackboard(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM blackboards WHERE id=$1::uuid`, id)
    if err != nil { return 0, err }
    return ct.RowsAffected(), nil
}

