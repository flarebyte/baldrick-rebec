package version

import (
    "encoding/json"
    "fmt"
    "os"
    "runtime"
    "time"

    "github.com/flarebyte/baldrick-rebec/internal/buildinfo"
    "github.com/spf13/cobra"
)

var (
    flagShort bool
)

var VersionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print the CLI version",
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagShort {
            fmt.Fprintln(os.Stdout, buildinfo.Summary())
            return nil
        }
        // Human friendly line to stderr
        fmt.Fprintf(os.Stderr, "rbc version: %s\n", buildinfo.Summary())
        // JSON to stdout
        out := map[string]any{
            "version":   buildinfo.Version,
            "commit":    buildinfo.Commit,
            "date":      buildinfo.Date,
            "built_by":  buildinfo.BuiltBy,
            "go":        runtime.Version(),
            "go_os":     runtime.GOOS,
            "go_arch":   runtime.GOARCH,
            "timestamp": time.Now().UTC().Format(time.RFC3339Nano),
        }
        enc := json.NewEncoder(os.Stdout)
        enc.SetIndent("", "  ")
        return enc.Encode(out)
    },
}

func init() {
    VersionCmd.Flags().BoolVar(&flagShort, "short", false, "Print only the version string")
}

