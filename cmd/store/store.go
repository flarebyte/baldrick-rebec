package store

import "github.com/spf13/cobra"

var StoreCmd = &cobra.Command{
	Use:   "store",
	Short: "Manage stores (ideas, journals, blackboards) scoped by role",
}
