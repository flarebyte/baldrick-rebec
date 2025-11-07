package db

import (
    "context"
    "errors"
    "fmt"
    "os"
    "sort"
    "strings"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    tt "text/tabwriter"
    "github.com/spf13/cobra"
    "github.com/jackc/pgx/v5/pgxpool"
)

var (
    flagShowOutput string
    flagShowSchema string
    flagShowConcise bool
)

type columnInfo struct {
    Name     string
    DataType string
    Nullable string
    Default  string
    PK       bool
}

var showCmd = &cobra.Command{
    Use:   "show",
    Short: "Show database table schemas",
    RunE: func(cmd *cobra.Command, args []string) error {
        outFmt := strings.ToLower(strings.TrimSpace(flagShowOutput))
        if outFmt == "" { outFmt = "tables" }
        if outFmt != "tables" && outFmt != "md" {
            return errors.New("--output must be 'tables' or 'md'")
        }
        schema := flagShowSchema
        if strings.TrimSpace(schema) == "" { schema = "public" }

        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()

        // Fetch tables in the target schema
        tblRows, err := db.Query(ctx, `SELECT table_name FROM information_schema.tables
            WHERE table_schema=$1 AND table_type='BASE TABLE'
            ORDER BY table_name`, schema)
        if err != nil { return err }
        defer tblRows.Close()
        tables := []string{}
        for tblRows.Next() {
            var t string
            if err := tblRows.Scan(&t); err != nil { return err }
            tables = append(tables, t)
        }
        if err := tblRows.Err(); err != nil { return err }

        // stable order, and prefer our core tables first if present
        priority := map[string]int{
            "roles":0,"workflows":1,"tags":2,"projects":3,"stores":4,
            "conversations":5,"experiments":6,"task_variants":7,"tasks":8,
            "scripts_content":9,"scripts":10,"messages_content":11,"messages":12,
            "workspaces":13,"packages":14,"queues":15,"testcases":16,
        }
        sort.Slice(tables, func(i, j int) bool {
            pi, pj := 1000, 1000
            if v, ok := priority[tables[i]]; ok { pi = v }
            if v, ok := priority[tables[j]]; ok { pj = v }
            if pi != pj { return pi < pj }
            return tables[i] < tables[j]
        })

        // Output
        switch outFmt {
        case "tables":
            return showAsTables(ctx, db, schema, tables, flagShowConcise)
        case "md":
            return showAsMarkdown(ctx, db, schema, tables, flagShowConcise)
        default:
            return nil
        }
    },
}

func init() {
    DBCmd.AddCommand(showCmd)
    showCmd.Flags().StringVar(&flagShowOutput, "output", "tables", "Output format: tables or md")
    showCmd.Flags().StringVar(&flagShowSchema, "schema", "public", "Schema to inspect (default public)")
    showCmd.Flags().BoolVar(&flagShowConcise, "concise", false, "Concise view (columns and types only)")
}

func fetchColumns(ctx context.Context, db *pgxpool.Pool, schema, table string) ([]columnInfo, error) {
    // Primary key columns
    pkset := map[string]bool{}
    pkRows, err := db.Query(ctx, `SELECT kcu.column_name
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
          ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
        WHERE tc.table_schema=$1 AND tc.table_name=$2 AND tc.constraint_type='PRIMARY KEY'`, schema, table)
    if err == nil {
        defer pkRows.Close()
        for pkRows.Next() {
            var col string
            if err := pkRows.Scan(&col); err == nil {
                pkset[col] = true
            }
        }
        _ = pkRows.Err()
    }

    // Columns meta
    rows, err := db.Query(ctx, `SELECT column_name, data_type, is_nullable, COALESCE(column_default,'')
        FROM information_schema.columns
        WHERE table_schema=$1 AND table_name=$2
        ORDER BY ordinal_position`, schema, table)
    if err != nil { return nil, err }
    defer rows.Close()
    cols := []columnInfo{}
    for rows.Next() {
        var c columnInfo
        if err := rows.Scan(&c.Name, &c.DataType, &c.Nullable, &c.Default); err != nil { return nil, err }
        c.PK = pkset[c.Name]
        cols = append(cols, c)
    }
    return cols, rows.Err()
}

func showAsTables(ctx context.Context, db *pgxpool.Pool, schema string, tables []string, concise bool) error {
    for i, t := range tables {
        cols, err := fetchColumns(ctx, db, schema, t)
        if err != nil { return err }
        fmt.Fprintf(os.Stdout, "TABLE: %s\n", t)
        tw := tt.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
        if concise {
            fmt.Fprintln(tw, "COLUMN\tTYPE")
            for _, c := range cols {
                fmt.Fprintf(tw, "%s\t%s\n", c.Name, c.DataType)
            }
        } else {
            fmt.Fprintln(tw, "COLUMN\tTYPE\tNULL\tDEFAULT\tPK")
            for _, c := range cols {
                pk := ""; if c.PK { pk = "yes" }
                fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", c.Name, c.DataType, strings.ToLower(c.Nullable), c.Default, pk)
            }
        }
        tw.Flush()
        if i < len(tables)-1 { fmt.Fprintln(os.Stdout) }
    }
    return nil
}

func showAsMarkdown(ctx context.Context, db *pgxpool.Pool, schema string, tables []string, concise bool) error {
    for _, t := range tables {
        fmt.Fprintf(os.Stdout, "## %s\n", t)
        if concise {
            fmt.Fprintln(os.Stdout, "| Column | Type |")
            fmt.Fprintln(os.Stdout, "|---|---|")
        } else {
            fmt.Fprintln(os.Stdout, "| Column | Type | Nullable | Default | PK |")
            fmt.Fprintln(os.Stdout, "|---|---|---|---|---|")
        }
        cols, err := fetchColumns(ctx, db, schema, t)
        if err != nil { return err }
        for _, c := range cols {
            if concise {
                fmt.Fprintf(os.Stdout, "| %s | %s |\n", c.Name, c.DataType)
            } else {
                pk := ""; if c.PK { pk = "yes" }
                fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n", c.Name, c.DataType, strings.ToLower(c.Nullable), c.Default, pk)
            }
        }
        fmt.Fprintln(os.Stdout)
    }
    return nil
}
