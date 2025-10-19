package configcmd

import (
    "fmt"
    "os"

    cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/flarebyte/baldrick-rebec/internal/paths"
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

var (
    flagClearDryRun bool
)

var clearAdminTempCmd = &cobra.Command{
    Use:   "clear-admin-temp",
    Short: "Clear temporary admin passwords (PostgreSQL)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if _, err := paths.EnsureHome(); err != nil { return err }
        cfg, err := cfgpkg.Load()
        if err != nil { return err }

        // Clear temp admin passwords
        cfg.Postgres.Admin.PasswordTemp = ""

        b, err := yaml.Marshal(cfg)
        if err != nil { return err }
        if flagClearDryRun {
            os.Stdout.Write(b)
            if len(b) == 0 || b[len(b)-1] != '\n' { fmt.Fprintln(os.Stdout) }
            fmt.Fprintf(os.Stderr, "dry-run: not writing %s\n", cfgpkg.Path())
            return nil
        }
        if err := os.WriteFile(cfgpkg.Path(), b, 0o644); err != nil { return err }
        fmt.Fprintf(os.Stderr, "cleared temporary admin passwords in %s\n", cfgpkg.Path())
        return nil
    },
}

func init() {
    ConfigCmd.AddCommand(clearAdminTempCmd)
    clearAdminTempCmd.Flags().BoolVar(&flagClearDryRun, "dry-run", false, "Print merged config to stdout without writing")
}
