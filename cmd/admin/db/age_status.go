package db

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
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

        // Which DB/user and versions
        status["db"] = cfg.Postgres.DBName
        var currentUser string
        _ = db.QueryRow(ctx, "SELECT current_user").Scan(&currentUser)
        status["user"] = currentUser
        var pgver, agever string
        _ = db.QueryRow(ctx, "SHOW server_version").Scan(&pgver)
        _ = db.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname='age'").Scan(&agever)
        status["postgres_version"] = pgver
        status["age_version"] = agever

        // Check extension installed
        var extInstalled bool
        if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='age')").Scan(&extInstalled); err != nil {
            status["extension_error"] = err.Error()
        }
        status["extension_installed"] = extInstalled

        // Privileges on schemas (best-effort)
        var hasAgUsage, hasGraphUsage bool
        _ = db.QueryRow(ctx, "SELECT has_schema_privilege(current_user, 'ag_catalog', 'USAGE')").Scan(&hasAgUsage)
        _ = db.QueryRow(ctx, "SELECT has_schema_privilege(current_user, 'rbc_graph', 'USAGE')").Scan(&hasGraphUsage)
        status["has_ag_usage"] = hasAgUsage
        status["has_graph_usage"] = hasGraphUsage

        // Try a trivial cypher query to infer graph existence and usability
        var cypherOK bool
        var graphExists bool
        if extInstalled {
            q := "SELECT 1 FROM ag_catalog.cypher('rbc_graph', $$ RETURN 1 $$) as (x ag_catalog.agtype) LIMIT 1"
            if err := db.QueryRow(ctx, q).Scan(new(int)); err == nil {
                cypherOK = true
                graphExists = true
            } else {
                status["cypher_error"] = err.Error()
                // If error mentions graph not existing, flag as false; otherwise leave as unknown/false
                if strings.Contains(strings.ToLower(err.Error()), "does not exist") || strings.Contains(strings.ToLower(err.Error()), "graph") {
                    graphExists = false
                }
            }
        }
        status["cypher_usable"] = cypherOK
        status["graph_exists"] = graphExists

        // Operator presence checks in ag_catalog (best-effort)
        var hasAgtypeContains bool
        var hasGraphidEq bool
        if extInstalled {
            // @> operator in ag_catalog (any signature)
            _ = db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_operator WHERE oprnamespace='ag_catalog'::regnamespace AND oprname='@>')").Scan(&hasAgtypeContains)
            // graphid equals operator
            _ = db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_operator WHERE oprnamespace='ag_catalog'::regnamespace AND oprname='=' AND oprleft='ag_catalog.graphid'::regtype AND oprright='ag_catalog.graphid'::regtype)").Scan(&hasGraphidEq)
        }
        status["agtype_contains_operator"] = hasAgtypeContains
        status["graphid_eq_operator"] = hasGraphidEq

        // Human summary to stderr
        fmt.Fprintf(os.Stderr, "AGE extension installed: %v\n", extInstalled)
        fmt.Fprintf(os.Stderr, "Graph rbc_graph exists: %v\n", graphExists)
        fmt.Fprintf(os.Stderr, "ag_catalog USAGE: %v\n", hasAgUsage)
        fmt.Fprintf(os.Stderr, "rbc_graph USAGE: %v\n", hasGraphUsage)
        fmt.Fprintf(os.Stderr, "Cypher usable: %v\n", cypherOK)
        if agever != "" || pgver != "" {
            fmt.Fprintf(os.Stderr, "Versions: Postgres=%s AGE=%s\n", pgver, agever)
        }
        fmt.Fprintf(os.Stderr, "Operators: agtype @> present=%v, graphid = present=%v\n", hasAgtypeContains, hasGraphidEq)

        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(status)
    },
}

func init() {
    DBCmd.AddCommand(ageStatusCmd)
}
