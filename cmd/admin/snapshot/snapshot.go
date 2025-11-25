package snapshot

import "github.com/spf13/cobra"

// SnapshotCmd is the root for snapshot subcommands.
var SnapshotCmd = &cobra.Command{
    Use:   "snapshot",
    Short: "Manage logical backups (schema-aware) in a dedicated schema",
}

