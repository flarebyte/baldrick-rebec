package buildinfo

import "github.com/flarebyte/baldrick-rebec/cli"

// Package buildinfo exposes version metadata for the CLI. Values can be
// overridden at build time via -ldflags. This package also honors values set in
// the cli package (cli.Version/cli.Date) for compatibility with external build scripts.

var (
    // Version is the semantic version or custom string. Defaults to cli.Version or "dev".
    Version = "dev"
    // Commit is the VCS commit hash (optional).
    Commit  = ""
    // Date is the build time in RFC3339 or similar (optional). Falls back to cli.Date.
    Date    = ""
    // BuiltBy is an optional builder identifier (optional).
    BuiltBy = ""
)

// Summary returns a concise single-line version string.
func Summary() string {
    v := Version
    if v == "" {
        v = cli.Version
    }
    if v == "" {
        v = "dev"
    }
    if len(Commit) >= 7 {
        v += " (" + Commit[:7] + ")"
    }
    return v
}

