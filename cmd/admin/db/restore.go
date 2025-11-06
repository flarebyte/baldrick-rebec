package db

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/spf13/cobra"
)

var (
    flagRestoreInput   string
    flagRestoreDelete  bool
    flagRestoreUpsert  bool
)

type rowObj map[string]any

var restoreCmd = &cobra.Command{
    Use:   "restore",
    Short: "Restore tables from a JSON backup (stdin or file)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if !flagRestoreDelete && !flagRestoreUpsert { flagRestoreUpsert = true }
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()
        var r io.Reader = os.Stdin
        if strings.TrimSpace(flagRestoreInput) != "" {
            f, err := os.Open(flagRestoreInput)
            if err != nil { return err }
            defer f.Close(); r = f
        }
        var dump map[string][]json.RawMessage
        dec := json.NewDecoder(r)
        if err := dec.Decode(&dump); err != nil { return err }
        if flagRestoreDelete {
            if err := truncateAll(ctx, db); err != nil { return err }
        }
        // Insert in FK-safe order
        order := []string{"roles","workflows","tags","projects","stores","conversations","experiments","task_variants","tasks","scripts_content","scripts","messages_content","messages","workspaces","blackboards","packages","testcases"}
        for _, tbl := range order {
            rows := dump[tbl]
            for _, raw := range rows {
                var obj rowObj
                if err := json.Unmarshal(raw, &obj); err != nil { return fmt.Errorf("unmarshal %s: %w", tbl, err) }
                if err := upsertRow(ctx, db, tbl, obj, flagRestoreUpsert); err != nil { return fmt.Errorf("restore %s: %w", tbl, err) }
            }
        }
        fmt.Fprintln(os.Stderr, "db:restore - completed")
        return nil
    },
}

func truncateAll(ctx context.Context, db *pgxpool.Pool) error {
    _, err := db.Exec(ctx, `TRUNCATE TABLE packages, blackboards, messages, messages_content, scripts, scripts_content, tasks, task_variants, experiments, conversations, workspaces, stores, projects, workflows, roles, tags RESTART IDENTITY CASCADE`)
    return err
}

func upsertRow(ctx context.Context, db *pgxpool.Pool, tbl string, obj rowObj, upsert bool) error {
    switch tbl {
    case "roles":
        return insertGeneric(ctx, db, tbl, []col{{"name",""},{"title",""},{"description",""},{"created",":timestamptz"},{"updated",":timestamptz"},{"notes",""},{"tags",":jsonb"}}, "name", upsert, obj)
    case "workflows":
        return insertGeneric(ctx, db, tbl, []col{{"name",""},{"title",""},{"description",""},{"role_name",""},{"created",":timestamptz"},{"updated",":timestamptz"},{"notes",""}}, "name", upsert, obj)
    case "tags":
        return insertGeneric(ctx, db, tbl, []col{{"name",""},{"title",""},{"description",""},{"role_name",""},{"created",":timestamptz"},{"updated",":timestamptz"},{"notes",""}}, "name", upsert, obj)
    case "projects":
        return insertGeneric(ctx, db, tbl, []col{{"name",""},{"role_name",""},{"description",""},{"created",":timestamptz"},{"updated",":timestamptz"},{"notes",""},{"tags",":jsonb"}}, "(name,role_name)", upsert, obj)
    case "conversations":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"title",""},{"description",""},{"project",""},{"role_name",""},{"tags",":jsonb"},{"created",":timestamptz"},{"updated",":timestamptz"},{"notes",""}}, "id", upsert, obj)
    case "experiments":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"conversation_id",":uuid"},{"created",":timestamptz"}}, "id", upsert, obj)
    case "task_variants":
        return insertGeneric(ctx, db, tbl, []col{{"variant",""},{"workflow_id",""}}, "variant", upsert, obj)
    case "tasks":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"command",""},{"variant",""},{"title",""},{"description",""},{"motivation",""},{"role_name",""},{"created",":timestamptz"},{"notes",""},{"shell",""},{"run_script_id",":uuid"},{"timeout",":interval"},{"tool_workspace_id",":uuid"},{"tags",":jsonb"},{"level",""}}, "id", upsert, obj)
    case "scripts_content":
        // id is bytea (base64 in backup)
        if v, ok := obj["id"].(string); ok && v != "" {
            b, _ := base64.StdEncoding.DecodeString(v)
            obj["id"] = b
        }
        return insertGeneric(ctx, db, tbl, []col{{"id",":bytea"},{"script_content",""},{"created_at",":timestamptz"}}, "id", upsert, obj)
    case "scripts":
        // script_content_id is bytea (base64)
        if v, ok := obj["script_content_id"].(string); ok && v != "" {
            b, _ := base64.StdEncoding.DecodeString(v)
            obj["script_content_id"] = b
        }
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"title",""},{"description",""},{"motivation",""},{"notes",""},{"script_content_id",":bytea"},{"role_name",""},{"tags",":jsonb"},{"created",":timestamptz"},{"updated",":timestamptz"}}, "id", upsert, obj)
    case "messages_content":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"text_content",""},{"json_content",":jsonb"},{"created_at",":timestamptz"}}, "id", upsert, obj)
    case "messages":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"content_id",":uuid"},{"from_task_id",":uuid"},{"experiment_id",":uuid"},{"role_name",""},{"status",""},{"error_message",""},{"tags",":jsonb"},{"created",":timestamptz"}}, "id", upsert, obj)
    case "workspaces":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"description",""},{"role_name",""},{"project_name",""},{"build_script_id",":uuid"},{"created",":timestamptz"},{"updated",":timestamptz"},{"tags",":jsonb"}}, "id", upsert, obj)
    case "blackboards":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"store_id",":uuid"},{"role_name",""},{"conversation_id",":uuid"},{"project_name",""},{"task_id",":uuid"},{"created",":timestamptz"},{"updated",":timestamptz"},{"background",""},{"guidelines",""}}, "id", upsert, obj)
    case "packages":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"role_name",""},{"task_id",":uuid"},{"created",":timestamptz"},{"updated",":timestamptz"}}, "id", upsert, obj)
    case "testcases":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"name",""},{"package",""},{"classname",""},{"title",""},{"experiment_id",":uuid"},{"role_name",""},{"status",""},{"error_message",""},{"tags",":jsonb"},{"level",""},{"created",":timestamptz"},{"file",""},{"line",""},{"execution_time",""}}, "id", upsert, obj)
    case "stores":
        return insertGeneric(ctx, db, tbl, []col{{"id",":uuid"},{"name",""},{"title",""},{"description",""},{"motivation",""},{"security",""},{"privacy",""},{"role_name",""},{"created",":timestamptz"},{"updated",":timestamptz"},{"notes",""},{"tags",":jsonb"},{"store_type",""},{"scope",""},{"lifecycle",""}}, "id", upsert, obj)
    default:
        return errors.New("unknown table: "+tbl)
    }
}

type col struct{ name string; cast string }

func insertGeneric(ctx context.Context, db *pgxpool.Pool, tbl string, cols []col, conflict string, upsert bool, obj rowObj) error {
    names := make([]string, 0, len(cols))
    placeholders := make([]string, 0, len(cols))
    args := make([]any, 0, len(cols))
    for i, c := range cols {
        names = append(names, c.name)
        placeholders = append(placeholders, fmt.Sprintf("$%d%s", i+1, c.cast))
        // prepare param
        val, _ := obj[c.name]
        // jsonb: re-marshal to raw
        if c.cast == ":jsonb" && val != nil {
            b, _ := json.Marshal(val)
            val = string(b)
        }
        args = append(args, val)
    }
    q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tbl, strings.Join(names, ","), strings.Join(placeholders, ","))
    if upsert {
        if conflict == "(name,role_name)" {
            sets := make([]string, 0, len(cols))
            for _, c := range cols {
                if c.name == "name" || c.name == "role_name" { continue }
                sets = append(sets, fmt.Sprintf("%s=EXCLUDED.%s", c.name, c.name))
            }
            q += fmt.Sprintf(" ON CONFLICT %s DO UPDATE SET %s", conflict, strings.Join(sets, ","))
        } else {
            pk := conflict
            sets := make([]string, 0, len(cols))
            for _, c := range cols {
                // do not update primary key
                if pk == c.name { continue }
                sets = append(sets, fmt.Sprintf("%s=EXCLUDED.%s", c.name, c.name))
            }
            q += fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", pk, strings.Join(sets, ","))
        }
    } else {
        q += " ON CONFLICT DO NOTHING"
    }
    _, err := db.Exec(ctx, q, args...)
    return err
}

func init() {
    DBCmd.AddCommand(restoreCmd)
    restoreCmd.Flags().StringVar(&flagRestoreInput, "input", "", "Input JSON file (default stdin)")
    restoreCmd.Flags().BoolVar(&flagRestoreDelete, "delete-existing", false, "Delete existing records before restore")
    restoreCmd.Flags().BoolVar(&flagRestoreUpsert, "upsert", false, "Upsert (create/update) existing records (default true if no flags)")
}
