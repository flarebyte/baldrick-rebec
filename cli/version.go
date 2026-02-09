package cli

import "strings"

// Version and Date should be set at build time using ldflags, e.g.:
//   -ldflags "-X 'github.com/flarebyte/baldrick-rebec/cli.Version=1.2.3' -X 'github.com/flarebyte/baldrick-rebec/cli.Date=2026-02-09'"
var (
    Version string
    Date    string
)

// niceDate replaces dashes with spaces for nicer display.
var niceDate = strings.ReplaceAll(Date, "-", " ")

