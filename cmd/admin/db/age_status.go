package db

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "time"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
    "github.com/spf13/cobra"
)

var ageStatusCmd = &cobra.Command{
    Use:   "age-status",
    Short: "Report Apache AGE extension status, graph presence, and privileges",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load(); if err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        db, err := pgdao.OpenApp(ctx, cfg); if err != nil { return err }
        defer db.Close()

        status := map[string]any{}

        // Which DB/user
        status["db"] = cfg.Postgres.DBName
        // Attempt to get current_user
        var currentUser string
        _ = db.QueryRow(ctx, "SELECT current_user").Scan(&currentUser)
        status["user"] = currentUser

        // Check extension installed
        var extInstalled bool
        if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='age')").Scan(&extInstalled); err != nil {
            status["extension_error"] = err.Error()
        }
        status["extension_installed"] = extInstalled

        // Check graph exists (if ag_catalog available)
        var graphExists bool
        if extInstalled {
            if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM ag_catalog.ag_graph WHERE name='rbc_graph')").Scan(&graphExists); err != nil {
                // If ag_catalog is not accessible, record error
                status["graph_check_error"] = err.Error()
            }
        }
        status["graph_exists"] = graphExists

        // Privileges on schemas (best-effort)
        var hasAgUsage, hasGraphUsage bool
        _ = db.QueryRow(ctx, "SELECT has_schema_privilege(current_user, 'ag_catalog', 'USAGE')").Scan(&hasAgUsage)
        _ = db.QueryRow(ctx, "SELECT has_schema_privilege(current_user, 'rbc_graph', 'USAGE')").Scan(&hasGraphUsage)
        status["has_ag_usage"] = hasAgUsage
        status["has_graph_usage"] = hasGraphUsage

        // Can execute a trivial cypher query
        var cypherOK bool
        if extInstalled && graphExists {
            q := "SELECT 1 FROM ag_catalog.cypher('rbc_graph', $$ RETURN 1 $$) as (x ag_catalog.agtype) LIMIT 1"
            if err := db.QueryRow(ctx, q).Scan(new(int)); err == nil {
                cypherOK = true
            } else {
                status["cypher_error"] = err.Error()
            }
        }
        status["cypher_usable"] = cypherOK

        // Human summary to stderr
        fmt.Fprintf(os.Stderr, "AGE extension installed: %v\n", extInstalled)
        fmt.Fprintf(os.Stderr, "Graph rbc_graph exists: %v\n", graphExists)
        fmt.Fprintf(os.Stderr, "ag_catalog USAGE: %v\n", hasAgUsage)
        fmt.Fprintf(os.Stderr, "rbc_graph USAGE: %v\n", hasGraphUsage)
        fmt.Fprintf(os.Stderr, "Cypher usable: %v\n", cypherOK)

        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(status)
    },
}

func init() {
    DBCmd.AddCommand(ageStatusCmd)
}
