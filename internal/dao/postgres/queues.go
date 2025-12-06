package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queue struct {
	ID                string
	Description       sql.NullString
	InQueueSince      sql.NullTime
	Status            string
	Why               sql.NullString
	Tags              map[string]any
	TaskID            sql.NullString
	InboundMessageID  sql.NullString
	TargetWorkspaceID sql.NullString
}

func AddQueue(ctx context.Context, db *pgxpool.Pool, q *Queue) error {
	stmt := `INSERT INTO queues (description, status, why, tags, task_id, inbound_message, target_workspace_id)
             VALUES (NULLIF($1,''), COALESCE(NULLIF($2,''),'Waiting'), NULLIF($3,''), COALESCE($4,'{}'::jsonb),
                     CASE WHEN $5='' THEN NULL ELSE $5::uuid END,
                     CASE WHEN $6='' THEN NULL ELSE $6::uuid END,
                     CASE WHEN $7='' THEN NULL ELSE $7::uuid END)
             RETURNING id::text, inQueueSince`
	var tagsJSON []byte
	if q.Tags != nil {
		tagsJSON, _ = json.Marshal(q.Tags)
	}
	if err := db.QueryRow(ctx, stmt,
		stringOrEmpty(q.Description), q.Status, stringOrEmpty(q.Why), tagsJSON,
		stringOrEmpty(q.TaskID), stringOrEmpty(q.InboundMessageID), stringOrEmpty(q.TargetWorkspaceID),
	).Scan(&q.ID, &q.InQueueSince); err != nil {
		return dbutil.ErrWrap("queue.insert", err, dbutil.ParamSummary("status", q.Status))
	}
	return nil
}

// TakeQueue sets status to 'Running' and returns the row.
func TakeQueue(ctx context.Context, db *pgxpool.Pool, id string) (*Queue, error) {
	q := `UPDATE queues SET status='Running' WHERE id=$1::uuid
         RETURNING id::text, description, inQueueSince, status, why, tags, task_id::text, inbound_message::text, target_workspace_id::text`
	var out Queue
	var tagsJSON []byte
	if err := db.QueryRow(ctx, q, id).Scan(
		&out.ID, &out.Description, &out.InQueueSince, &out.Status, &out.Why, &tagsJSON, &out.TaskID, &out.InboundMessageID, &out.TargetWorkspaceID,
	); err != nil {
		return nil, dbutil.ErrWrap("queue.take", err, dbutil.ParamSummary("id", id))
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &out.Tags)
	}
	return &out, nil
}

// PeekQueues returns up to limit queues ordered by oldest.
func PeekQueues(ctx context.Context, db *pgxpool.Pool, limit int, status string) ([]Queue, error) {
	if limit <= 0 {
		limit = 10
	}
	var rows pgxRows
	var err error
	if stringsTrim(status) != "" {
		rows, err = db.Query(ctx, `SELECT id::text, description, inQueueSince, status, why, tags, task_id::text, inbound_message::text, target_workspace_id::text
                                   FROM queues WHERE status=$1 ORDER BY inQueueSince ASC LIMIT $2`, status, limit)
	} else {
		rows, err = db.Query(ctx, `SELECT id::text, description, inQueueSince, status, why, tags, task_id::text, inbound_message::text, target_workspace_id::text
                                   FROM queues ORDER BY inQueueSince ASC LIMIT $1`, limit)
	}
	if err != nil {
		return nil, dbutil.ErrWrap("queue.peek", err, dbutil.ParamSummary("status", status), fmt.Sprintf("limit=%d", limit))
	}
	defer rows.Close()
	var out []Queue
	for rows.Next() {
		var q Queue
		var tagsJSON []byte
		if stringsTrim(status) != "" {
			if err := rows.Scan(&q.ID, &q.Description, &q.InQueueSince, &q.Status, &q.Why, &tagsJSON, &q.TaskID, &q.InboundMessageID, &q.TargetWorkspaceID); err != nil {
				return nil, dbutil.ErrWrap("queue.peek.scan", err)
			}
		} else {
			if err := rows.Scan(&q.ID, &q.Description, &q.InQueueSince, &q.Status, &q.Why, &tagsJSON, &q.TaskID, &q.InboundMessageID, &q.TargetWorkspaceID); err != nil {
				return nil, dbutil.ErrWrap("queue.peek.scan", err)
			}
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &q.Tags)
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("queue.peek", err)
	}
	return out, nil
}

func CountQueues(ctx context.Context, db *pgxpool.Pool, status string) (int64, error) {
	if stringsTrim(status) != "" {
		var n int64
		if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM queues WHERE status=$1`, status).Scan(&n); err != nil {
			return 0, dbutil.ErrWrap("queue.count", err, dbutil.ParamSummary("status", status))
		}
		return n, nil
	}
	var n int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM queues`).Scan(&n); err != nil {
		return 0, dbutil.ErrWrap("queue.count", err)
	}
	return n, nil
}
