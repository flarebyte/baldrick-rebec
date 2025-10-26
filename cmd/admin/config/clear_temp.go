package configcmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var (
    flagClearDryRun bool
)

var clearAdminTempCmd = &cobra.Command{
    Use:    "clear-admin-temp",
    Short:  "Deprecated: no temp admin password is used",
    Hidden: true,
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Fprintln(os.Stderr, "Nothing to do: postgres.admin.password_temp has been removed.")
        return nil
    },
}

func init() {
    ConfigCmd.AddCommand(clearAdminTempCmd)
    clearAdminTempCmd.Flags().BoolVar(&flagClearDryRun, "dry-run", false, "Print merged config to stdout without writing")
}
