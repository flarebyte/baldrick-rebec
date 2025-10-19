package configcmd

import (
    "errors"
    "fmt"
    "os"
    "strings"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
    Use:   "validate",
    Short: "Validate configuration and report issues",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        var problems []string

        if cfg.Server.Port <= 0 { problems = append(problems, "server.port must be > 0") }
        if cfg.Postgres.Host == "" { problems = append(problems, "postgres.host is required") }
        if cfg.Postgres.Port <= 0 { problems = append(problems, "postgres.port must be > 0") }
        if cfg.Postgres.DBName == "" { problems = append(problems, "postgres.dbname is required") }

        // OpenSearch removed in PG-only mode; no validation

        if len(problems) > 0 {
            fmt.Fprintln(os.Stderr, "Configuration issues:")
            for _, p := range problems { fmt.Fprintf(os.Stderr, "- %s\n", p) }
            return errors.New(strings.Join(problems, "; "))
        }
        fmt.Fprintln(os.Stderr, "Configuration looks valid.")
        return nil
    },
}
