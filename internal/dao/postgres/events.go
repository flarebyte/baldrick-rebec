package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageEvent struct {
	ID           string
	ContentID    string
	FromTaskID   sql.NullString
	ExperimentID sql.NullString
	RoleName     string
	Created      time.Time
	Status       string
	ErrorMessage sql.NullString
	Tags         map[string]any
}

func InsertMessageEvent(ctx context.Context, db *pgxpool.Pool, ev *MessageEvent) (string, error) {
	if ev == nil {
		return "", errors.New("nil event")
	}
	q := `INSERT INTO messages (
            content_id, from_task_id, experiment_id, role_name,
            status, error_message, tags, created
        ) VALUES (
            $1::uuid,$2,$3,$4,
            $5,$6,COALESCE($7,'{}'::jsonb),COALESCE($8, now())
        ) RETURNING id::text`
	var id string
	var created any
	if ev.Created.IsZero() {
		created = nil
	} else {
		created = ev.Created
	}
	var tagsJSON []byte
	if ev.Tags != nil {
		tagsJSON, _ = json.Marshal(ev.Tags)
	}
	err := db.QueryRow(ctx, q,
		ev.ContentID, nullOrUUID(ev.FromTaskID), nullOrUUID(ev.ExperimentID), ev.RoleName,
		ev.Status, nullOrString(ev.ErrorMessage), tagsJSON, created,
	).Scan(&id)
	if err != nil {
		return "", dbutil.ErrWrap("message.insert", err,
			dbutil.ParamSummary("content_id", ev.ContentID), dbutil.ParamSummary("from_task_id", ev.FromTaskID), dbutil.ParamSummary("experiment_id", ev.ExperimentID), dbutil.ParamSummary("status", ev.Status))
	}
	ev.ID = id
	return id, nil
}

// UpdateMessageEvent updates mutable fields of a message event: status,
// error_message, content_id, tags. Any zero-value/empty inputs are ignored
// unless explicitly provided via sql.Null* with Valid=true.
func UpdateMessageEvent(ctx context.Context, db *pgxpool.Pool, id string, update MessageEvent) error {
	// Build a compact UPDATE with COALESCE on provided fields while staying parameterized.
	// We purposely keep a fixed-shape query to avoid dynamic SQL per DB guidelines.
	q := `UPDATE messages SET
            status = COALESCE(NULLIF($1,''), status),
            error_message = COALESCE($2, error_message),
            content_id = COALESCE(NULLIF($3::uuid,'00000000-0000-0000-0000-000000000000'::uuid), content_id),
            tags = COALESCE($4, tags)
          WHERE id=$5::uuid`
	var tagsJSON []byte
	if update.Tags != nil {
		tagsJSON, _ = json.Marshal(update.Tags)
	}
	_, err := db.Exec(ctx, q,
		update.Status,
		nullOrString(update.ErrorMessage),
		update.ContentID,
		jsonOrNil(tagsJSON),
		id,
	)
	return err
}

func GetMessageEventByID(ctx context.Context, db *pgxpool.Pool, id string) (*MessageEvent, error) {
	q := `SELECT id::text, content_id::text,
                 from_task_id, experiment_id,
                 created, status, error_message, tags
          FROM messages WHERE id=$1::uuid`
	row := db.QueryRow(ctx, q, id)
	var out MessageEvent
	var metaBytes []byte
	var tagsJSON []byte
	var taskID, expID sql.NullString
	err := row.Scan(
		&out.ID, &out.ContentID,
		&taskID, &expID,
		&out.Created, &out.Status, &out.ErrorMessage, &tagsJSON,
	)
	if err != nil {
		return nil, dbutil.ErrWrap("message.get", err, dbutil.ParamSummary("id", id))
	}
	out.FromTaskID = taskID
	out.ExperimentID = expID
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &out.Tags)
	}
	_ = metaBytes
	return &out, nil
}

// ListMessages lists messages with optional filters.
func ListMessages(ctx context.Context, db *pgxpool.Pool, roleName, experimentID, taskID string, status string, limit, offset int) ([]MessageEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	// Build simple filter branches to stay parameterized and avoid dynamic SQL
	var rows pgxRows
	var err error
	switch {
	case stringsTrim(experimentID) != "" && stringsTrim(taskID) != "" && status != "":
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 AND experiment_id=$2::uuid AND from_task_id=$3::uuid AND status=$4
                                   ORDER BY created DESC LIMIT $5 OFFSET $6`, roleName, experimentID, taskID, status, limit, offset)
	case stringsTrim(experimentID) != "" && stringsTrim(taskID) != "":
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 AND experiment_id=$2::uuid AND from_task_id=$3::uuid
                                   ORDER BY created DESC LIMIT $4 OFFSET $5`, roleName, experimentID, taskID, limit, offset)
	case stringsTrim(experimentID) != "" && status != "":
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 AND experiment_id=$2::uuid AND status=$3
                                   ORDER BY created DESC LIMIT $4 OFFSET $5`, roleName, experimentID, status, limit, offset)
	case stringsTrim(taskID) != "" && status != "":
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 AND from_task_id=$2::uuid AND status=$3
                                   ORDER BY created DESC LIMIT $4 OFFSET $5`, roleName, taskID, status, limit, offset)
	case stringsTrim(experimentID) != "":
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 AND experiment_id=$2::uuid
                                   ORDER BY created DESC LIMIT $3 OFFSET $4`, roleName, experimentID, limit, offset)
	case stringsTrim(taskID) != "":
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 AND from_task_id=$2::uuid
                                   ORDER BY created DESC LIMIT $3 OFFSET $4`, roleName, taskID, limit, offset)
	case status != "":
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 AND status=$2
                                   ORDER BY created DESC LIMIT $3 OFFSET $4`, roleName, status, limit, offset)
	default:
		rows, err = db.Query(ctx, `SELECT id::text, content_id::text, from_task_id, experiment_id, created, status, error_message, tags
                                   FROM messages WHERE role_name=$1 ORDER BY created DESC LIMIT $2 OFFSET $3`, roleName, limit, offset)
	}
	if err != nil {
		return nil, dbutil.ErrWrap("message.list", err, fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []MessageEvent
	for rows.Next() {
		var r MessageEvent
		var tagsJSON []byte
		if err := rows.Scan(&r.ID, &r.ContentID, &r.FromTaskID, &r.ExperimentID, &r.Created, &r.Status, &r.ErrorMessage, &tagsJSON); err != nil {
			return nil, dbutil.ErrWrap("message.list.scan", err)
		}
		if len(tagsJSON) > 0 {
			_ = json.Unmarshal(tagsJSON, &r.Tags)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("message.list", err)
	}
	return out, nil
}

// DeleteMessage deletes a message by id.
func DeleteMessage(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM messages WHERE id=$1::uuid`, id)
	if err != nil {
		return 0, dbutil.ErrWrap("message.delete", err, dbutil.ParamSummary("id", id))
	}
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

func stringOrEmpty(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
func nullOrUUID(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

// pgTextArrayOrNil returns nil when empty to avoid overriding with an empty array
// when the caller intended to leave the field unchanged.
func pgTextArrayOrNil(arr []string) any {
	if len(arr) == 0 {
		return nil
	}
	return arr
}

// jsonOrNil returns nil when the JSON payload is empty so the UPDATE can COALESCE.
func jsonOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	// Accept "{}" or other content as-is.
	return b
}

// nullOrFloat64 returns nil when the float is invalid (unset) to enable COALESCE in UPDATEs
func nullOrFloat64(nf sql.NullFloat64) any {
	if nf.Valid {
		return nf.Float64
	}
	return nil
}
