package snapshot

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    bkp "github.com/flarebyte/baldrick-rebec/internal/backup"
    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/olekukonko/tablewriter"
    "github.com/spf13/cobra"
    "github.com/jackc/pgx/v5/pgxpool"
)

var (
    flagVerifySchema string
    flagVerifyJSON   bool
)

type verifyRow struct {
    Entity      string `json:"entity"`
    LiveCount   int64  `json:"live_count"`
    BackupCount int64  `json:"backup_count"`
    Match       bool   `json:"match"`
    SchemaDiffs int    `json:"schema_diffs"`
}

var verifyCmd = &cobra.Command{
    Use:   "verify <backup-id>",
    Short: "Verify a backup against live schema and counts",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        id := args[0]
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute); defer cancel()
        db, err := pgdao.OpenBackup(ctx, cfg); if err != nil { return err }
        defer db.Close()

        // list entities in backup
        ents, err := pgdao.ListBackupEntities(ctx, db, flagVerifySchema, id)
        if err != nil { return err }
        // map config by entity
        conf := map[string]bkp.BackupEntityConfig{}
        for _, e := range bkp.DefaultEntities() { conf[e.EntityName] = e }

        var rows []verifyRow
        for _, name := range ents {
            cfgEnt, ok := conf[name]
            var live int64
            var backupCount int64
            var diffs int
            if ok {
                // Count live rows
                if n, err := pgdao.CountTable(ctx, db, "public", cfgEnt.TableName); err == nil {
                    live = n
                }
                // Schema diff: compare entity_schema fields vs information_schema
                if n, err := compareSchema(ctx, db, flagVerifySchema, id, name, cfgEnt.TableName); err == nil {
                    diffs = n
                }
            }
            // backup count
            if n, err := pgdao.CountBackupEntityRecords(ctx, db, flagVerifySchema, id, name); err == nil {
                backupCount = n
            }
            rows = append(rows, verifyRow{Entity: name, LiveCount: live, BackupCount: backupCount, Match: live == backupCount && live > 0, SchemaDiffs: diffs})
        }
        if flagVerifyJSON {
            return json.NewEncoder(os.Stdout).Encode(rows)
        }
        tw := tablewriter.NewWriter(os.Stdout)
        tw.SetHeader([]string{"ENTITY", "LIVE", "BACKUP", "MATCH", "SCHEMA_DIFFS"})
        for _, r := range rows {
            m := "no"; if r.Match { m = "yes" }
            tw.Append([]string{r.Entity, fmt.Sprintf("%d", r.LiveCount), fmt.Sprintf("%d", r.BackupCount), m, fmt.Sprintf("%d", r.SchemaDiffs)})
        }
        tw.Render()
        return nil
    },
}

func compareSchema(ctx context.Context, db *pgxpool.Pool, backupSchema, backupID, entity, liveTable string) (int, error) {
    // load entity_schema fields
    rows, err := db.Query(ctx, `SELECT field_name, data_type, is_nullable FROM `+backupSchema+`.entity_schema WHERE backup_id=$1 AND entity_name=$2`, backupID, entity)
    if err != nil { return 0, err }
    defer rows.Close()
    bfields := map[string]struct{ typ string; nullable bool }{}
    for rows.Next() {
        var name, typ string
        var nullable bool
        if err := rows.Scan(&name, &typ, &nullable); err != nil { return 0, err }
        bfields[name] = struct{typ string; nullable bool}{typ: typ, nullable: nullable}
    }
    // live fields
    lrows, err := db.Query(ctx, `SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name=$1`, liveTable)
    if err != nil { return 0, err }
    defer lrows.Close()
    lfields := map[string]struct{ typ string; nullable bool }{}
    for lrows.Next() {
        var name, typ, nullableStr string
        if err := lrows.Scan(&name, &typ, &nullableStr); err != nil { return 0, err }
        lfields[name] = struct{typ string; nullable bool}{typ: typ, nullable: nullableStr == "YES"}
    }
    // count differences (added/removed/changed)
    diffs := 0
    for k, bv := range bfields {
        if lv, ok := lfields[k]; !ok || lv.typ != bv.typ || lv.nullable != bv.nullable {
            diffs++
        }
    }
    for k := range lfields {
        if _, ok := bfields[k]; !ok { diffs++ }
    }
    return diffs, nil
}

func init() {
    SnapshotCmd.AddCommand(verifyCmd)
    verifyCmd.Flags().StringVar(&flagVerifySchema, "schema", "backup", "Backup schema name")
    verifyCmd.Flags().BoolVar(&flagVerifyJSON, "json", false, "Output JSON")
}
