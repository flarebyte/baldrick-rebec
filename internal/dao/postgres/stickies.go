package postgres

import (
	"context"
	"database/sql"
	"fmt"

	dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Stickie struct {
	ID           string
	BlackboardID string
	// topic fields removed; use labels instead
	Note            sql.NullString
	Code            sql.NullString
	Labels          []string
	Created         sql.NullTime
	Updated         sql.NullTime
	CreatedByTaskID sql.NullString
	EditCount       int
	PriorityLevel   sql.NullString
	Score           sql.NullFloat64
	Name            sql.NullString
	Archived        bool
}

// UpsertStickie inserts a new stickie if ID is empty, otherwise updates it.
func UpsertStickie(ctx context.Context, db *pgxpool.Pool, s *Stickie) error {
	if s.ID != "" {
		q := `UPDATE stickies
              SET blackboard_id=CASE WHEN $2='' THEN blackboard_id ELSE $2::uuid END,
                  note=COALESCE(NULLIF($3,''), note),
                  code=COALESCE(NULLIF($4,''), code),
                  labels=COALESCE($5, labels),
                  created_by_task_id=COALESCE(CASE WHEN $6='' THEN NULL ELSE $6::uuid END, created_by_task_id),
                  priority_level=COALESCE(NULLIF($7,''), priority_level),
                  name=COALESCE(NULLIF($8,''), name),
                  archived=$9,
                  score=COALESCE($10::double precision, score)
              WHERE id=$1::uuid
              RETURNING created, updated, edit_count`
		if err := db.QueryRow(ctx, q,
			s.ID, s.BlackboardID, nullOrString(s.Note), stringOrEmpty(s.Code), pgTextArrayOrNil(s.Labels), nullOrUUID(s.CreatedByTaskID), nullOrString(s.PriorityLevel), nullOrString(s.Name), s.Archived, nullOrFloat64(s.Score),
		).Scan(&s.Created, &s.Updated, &s.EditCount); err != nil {
			return dbutil.ErrWrap("stickie.upsert.update", err,
				dbutil.ParamSummary("id", s.ID), dbutil.ParamSummary("blackboard_id", s.BlackboardID))
		}
		return nil
	}
	q := `INSERT INTO stickies (blackboard_id, note, code, labels, created_by_task_id, priority_level, name, archived, score)
          VALUES ($1::uuid, NULLIF($2,''), NULLIF($3,''), COALESCE($4,ARRAY[]::text[]), CASE WHEN $5='' THEN NULL ELSE $5::uuid END, NULLIF($6,''), NULLIF($7,''), COALESCE($8,false), $9::double precision)
          RETURNING id::text, created, updated, edit_count`
	if err := db.QueryRow(ctx, q,
		s.BlackboardID, stringOrEmpty(s.Note), stringOrEmpty(s.Code), pgTextArrayOrNil(s.Labels), stringOrEmpty(s.CreatedByTaskID), stringOrEmpty(s.PriorityLevel), stringOrEmpty(s.Name), s.Archived, nullOrFloat64(s.Score),
	).Scan(&s.ID, &s.Created, &s.Updated, &s.EditCount); err != nil {
		return dbutil.ErrWrap("stickie.upsert.insert", err,
			dbutil.ParamSummary("blackboard_id", s.BlackboardID))
	}
	return nil
}

// GetStickieByID fetches a stickie by UUID.
func GetStickieByID(ctx context.Context, db *pgxpool.Pool, id string) (*Stickie, error) {
	q := `SELECT id::text, blackboard_id::text, note, code, labels, created, updated, created_by_task_id::text, edit_count, priority_level, score, name, archived
          FROM stickies WHERE id=$1::uuid`
	var s Stickie
	if err := db.QueryRow(ctx, q, id).Scan(&s.ID, &s.BlackboardID, &s.Note, &s.Code, &s.Labels, &s.Created, &s.Updated, &s.CreatedByTaskID, &s.EditCount, &s.PriorityLevel, &s.Score, &s.Name, &s.Archived); err != nil {
		return nil, dbutil.ErrWrap("stickie.get", err, dbutil.ParamSummary("id", id))
	}
	return &s, nil
}

// ListStickies lists stickies with optional filters.
func ListStickies(ctx context.Context, db *pgxpool.Pool, blackboardID string, limit, offset int) ([]Stickie, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var rows pgxRows
	var err error
	switch {
	case stringsTrim(blackboardID) != "":
		rows, err = db.Query(ctx, `SELECT id::text, blackboard_id::text, note, code, labels, created, updated, created_by_task_id::text, edit_count, priority_level, score, name, archived
                                   FROM stickies WHERE blackboard_id=$1::uuid
                                   ORDER BY updated DESC, created DESC LIMIT $2 OFFSET $3`, blackboardID, limit, offset)
	default:
		rows, err = db.Query(ctx, `SELECT id::text, blackboard_id::text, note, code, labels, created, updated, created_by_task_id::text, edit_count, priority_level, score, name, archived
                                   FROM stickies ORDER BY updated DESC, created DESC LIMIT $1 OFFSET $2`, limit, offset)
	}
	if err != nil {
		return nil, dbutil.ErrWrap("stickie.list", err,
			dbutil.ParamSummary("blackboard_id", blackboardID), fmt.Sprintf("limit=%d", limit), fmt.Sprintf("offset=%d", offset))
	}
	defer rows.Close()
	var out []Stickie
	for rows.Next() {
		var s Stickie
		if err := rows.Scan(&s.ID, &s.BlackboardID, &s.Note, &s.Code, &s.Labels, &s.Created, &s.Updated, &s.CreatedByTaskID, &s.EditCount, &s.PriorityLevel, &s.Score, &s.Name, &s.Archived); err != nil {
			return nil, dbutil.ErrWrap("stickie.list.scan", err, dbutil.ParamSummary("blackboard_id", blackboardID))
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, dbutil.ErrWrap("stickie.list", err, dbutil.ParamSummary("blackboard_id", blackboardID))
	}
	return out, nil
}

// DeleteStickie deletes by id.
func DeleteStickie(ctx context.Context, db *pgxpool.Pool, id string) (int64, error) {
	ct, err := db.Exec(ctx, `DELETE FROM stickies WHERE id=$1::uuid`, id)
	if err != nil {
		return 0, dbutil.ErrWrap("stickie.delete", err, dbutil.ParamSummary("id", id))
	}
	return ct.RowsAffected(), nil
}

// GetStickieByComplexName performs exact lookup on (complex_name.name, complex_name.variant) with archived flag.
func GetStickieByName(ctx context.Context, db *pgxpool.Pool, name string, archived bool) (*Stickie, error) {
	const q = `
        SELECT id::text, blackboard_id::text, note, code, labels,
               created, updated, created_by_task_id::text, edit_count, priority_level, score, name, archived
        FROM stickies
        WHERE name = $1
          AND archived = $2
        ORDER BY updated DESC
        LIMIT 1`
	var s Stickie
	if err := db.QueryRow(ctx, q, name, archived).
		Scan(&s.ID, &s.BlackboardID, &s.Note, &s.Code, &s.Labels,
			&s.Created, &s.Updated, &s.CreatedByTaskID, &s.EditCount, &s.PriorityLevel, &s.Score, &s.Name, &s.Archived); err != nil {
		return nil, dbutil.ErrWrap("stickie.get_by_name", err, dbutil.ParamSummary("name", name), dbutil.ParamSummary("archived", archived))
	}
	return &s, nil
}

// GetStickieByComplexNameInBlackboard exact lookup constrained by blackboard UUID.
func GetStickieByNameInBlackboard(ctx context.Context, db *pgxpool.Pool, name string, archived bool, blackboardID string) (*Stickie, error) {
	const q = `
        SELECT id::text, blackboard_id::text, note, code, labels,
               created, updated, created_by_task_id::text, edit_count, priority_level, score, name, archived
        FROM stickies
        WHERE blackboard_id = $1::uuid
          AND name = $2
          AND archived = $3
        ORDER BY updated DESC
        LIMIT 1`
	var s Stickie
	if err := db.QueryRow(ctx, q, blackboardID, name, archived).
		Scan(&s.ID, &s.BlackboardID, &s.Note, &s.Code, &s.Labels,
			&s.Created, &s.Updated, &s.CreatedByTaskID, &s.EditCount, &s.PriorityLevel, &s.Score, &s.Name, &s.Archived); err != nil {
		return nil, dbutil.ErrWrap("stickie.get_by_name_board", err, dbutil.ParamSummary("board", blackboardID), dbutil.ParamSummary("name", name), dbutil.ParamSummary("archived", archived))
	}
	return &s, nil
}
