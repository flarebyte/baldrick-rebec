package pkg

import "github.com/spf13/cobra"

var PackageCmd = &cobra.Command{
    Use:   "package",
    Short: "Manage packages (role-bound task selectors)",
}

