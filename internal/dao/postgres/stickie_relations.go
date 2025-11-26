package postgres

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
    dbutil "github.com/flarebyte/baldrick-rebec/internal/dao/dbutil"
)

type StickieRelation struct {
    FromID  string
    ToID    string
    RelType string
    Labels  []string
}

func UpsertStickieRelation(ctx context.Context, db *pgxpool.Pool, r StickieRelation) error {
    q := `INSERT INTO stickie_relations (from_id, to_id, rel_type, labels)
          VALUES ($1::uuid,$2::uuid,$3,COALESCE($4, ARRAY[]::text[]))
          ON CONFLICT (from_id,to_id,rel_type) DO UPDATE SET labels=EXCLUDED.labels`
    _, err := db.Exec(ctx, q, r.FromID, r.ToID, r.RelType, pgTextArrayOrNil(r.Labels))
    return dbutil.ErrWrap("stickie_rel.upsert", err, dbutil.ParamSummary("from", r.FromID), dbutil.ParamSummary("to", r.ToID), dbutil.ParamSummary("type", r.RelType))
}

func DeleteStickieRelation(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string) (int64, error) {
    ct, err := db.Exec(ctx, `DELETE FROM stickie_relations WHERE from_id=$1::uuid AND to_id=$2::uuid AND rel_type=$3`, fromID, toID, relType)
    if err != nil { return 0, dbutil.ErrWrap("stickie_rel.delete", err, dbutil.ParamSummary("from", fromID), dbutil.ParamSummary("to", toID), dbutil.ParamSummary("type", relType)) }
    return ct.RowsAffected(), nil
}

func GetStickieRelation(ctx context.Context, db *pgxpool.Pool, fromID, toID, relType string) (*StickieRelation, error) {
    q := `SELECT from_id::text, to_id::text, rel_type, labels FROM stickie_relations WHERE from_id=$1::uuid AND to_id=$2::uuid AND rel_type=$3`
    var r StickieRelation
    if err := db.QueryRow(ctx, q, fromID, toID, relType).Scan(&r.FromID, &r.ToID, &r.RelType, &r.Labels); err != nil {
        return nil, dbutil.ErrWrap("stickie_rel.get", err, dbutil.ParamSummary("from", fromID), dbutil.ParamSummary("to", toID), dbutil.ParamSummary("type", relType))
    }
    return &r, nil
}

func ListStickieRelations(ctx context.Context, db *pgxpool.Pool, id, dir string) ([]StickieRelation, error) {
    var rows pgxRows
    var err error
    switch dir {
    case "in":
        rows, err = db.Query(ctx, `SELECT from_id::text, to_id::text, rel_type, labels FROM stickie_relations WHERE to_id=$1::uuid`, id)
    case "both":
        rows, err = db.Query(ctx, `SELECT from_id::text, to_id::text, rel_type, labels FROM stickie_relations WHERE from_id=$1::uuid
                                    UNION ALL
                                    SELECT from_id::text, to_id::text, rel_type, labels FROM stickie_relations WHERE to_id=$1::uuid`, id)
    default:
        rows, err = db.Query(ctx, `SELECT from_id::text, to_id::text, rel_type, labels FROM stickie_relations WHERE from_id=$1::uuid`, id)
    }
    if err != nil { return nil, dbutil.ErrWrap("stickie_rel.list", err, dbutil.ParamSummary("id", id), dbutil.ParamSummary("dir", dir)) }
    defer rows.Close()
    out := []StickieRelation{}
    for rows.Next() {
        var r StickieRelation
        if err := rows.Scan(&r.FromID, &r.ToID, &r.RelType, &r.Labels); err != nil { return nil, dbutil.ErrWrap("stickie_rel.list.scan", err) }
        out = append(out, r)
    }
    if err := rows.Err(); err != nil { return nil, dbutil.ErrWrap("stickie_rel.list", err) }
    return out, nil
}
