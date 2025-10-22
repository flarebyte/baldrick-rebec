package configcmd

import (
    "errors"
    "fmt"
    "os"
    "strings"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/spf13/cobra"
)

var flagPasswords bool

var checkCmd = &cobra.Command{
    Use:   "check",
    Short: "Check configuration and report issues",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := cfgpkg.Load()
        if err != nil { return err }
        var problems []string

        if cfg.Server.Port <= 0 { problems = append(problems, "server.port must be > 0") }
        if cfg.Postgres.Host == "" { problems = append(problems, "postgres.host is required") }
        if cfg.Postgres.Port <= 0 { problems = append(problems, "postgres.port must be > 0") }
        if cfg.Postgres.DBName == "" { problems = append(problems, "postgres.dbname is required") }

        if flagPasswords {
            // Only show presence/absence, not values
            fmt.Fprintln(os.Stderr, "Password fields status (set=non-empty):")
            adminUser := cfg.Postgres.Admin.User
            if adminUser == "" { adminUser = "<unset>" }
            fmt.Fprintf(os.Stderr, "- postgres.admin.user: %s\n", adminUser)
            fmt.Fprintf(os.Stderr, "- postgres.admin.password: %v\n", cfg.Postgres.Admin.Password != "")
            fmt.Fprintf(os.Stderr, "- postgres.app.password: %v\n", cfg.Postgres.App.Password != "")
            // Legacy removed; no additional checks
        }

        if len(problems) > 0 {
            fmt.Fprintln(os.Stderr, "Configuration issues:")
            for _, p := range problems { fmt.Fprintf(os.Stderr, "- %s\n", p) }
            return errors.New(strings.Join(problems, "; "))
        }
        fmt.Fprintln(os.Stderr, "Configuration looks valid.")
        return nil
    },
}

func init() {
    checkCmd.Flags().BoolVar(&flagPasswords, "passwords", false, "Report which password fields are set (non-empty)")
}
