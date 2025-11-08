package db

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "regexp"
    "sort"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var (
    flagCountJSON bool
)

var countCmd = &cobra.Command{
    Use:   "count",
    Short: "Count rows for each public table",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
        defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg)
        if err != nil { return err }
        defer db.Close()

        // List public base tables
        rows, err := db.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name`)
        if err != nil { return err }
        defer rows.Close()
        var tables []string
        for rows.Next() {
            var name string
            if err := rows.Scan(&name); err != nil { return err }
            tables = append(tables, name)
        }
        if err := rows.Err(); err != nil { return err }

        // Count rows per table
        counts := map[string]int64{}
        identRe := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
        for _, t := range tables {
            if !identRe.MatchString(t) { continue }
            var n int64
            q := fmt.Sprintf("SELECT COUNT(*) FROM public.%s", t)
            if err := db.QueryRow(ctx, q).Scan(&n); err != nil { return err }
            counts[t] = n
        }

        // Try to include AGE graph edge counts (best-effort)
        edgeTypes := []string{"INCLUDES","CAUSES","USES","REPRESENTS","CONTRASTS_WITH"}
        for _, et := range edgeTypes {
            q := fmt.Sprintf("SELECT count(1) FROM ag_catalog.cypher('rbc_graph', $$ MATCH ()-[:%s]->() RETURN 1 $$) as (x ag_catalog.agtype)", et)
            var en int64
            if err := db.QueryRow(ctx, q).Scan(&en); err == nil {
                counts["graph_edges_"+et] = en
            }
        }

        // Human-readable to stderr
        keys := make([]string, 0, len(counts))
        for k := range counts { keys = append(keys, k) }
        sort.Strings(keys)
        for _, k := range keys {
            fmt.Fprintf(os.Stderr, "%s\t%d\n", k, counts[k])
        }

        // JSON to stdout
        enc := json.NewEncoder(os.Stdout)
        if flagCountJSON { enc.SetIndent("", "  ") }
        return enc.Encode(counts)
    },
}

func init() {
    DBCmd.AddCommand(countCmd)
    countCmd.Flags().BoolVar(&flagCountJSON, "json", false, "Pretty-print JSON output")
}
