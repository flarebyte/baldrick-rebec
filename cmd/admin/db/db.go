package db

import (
	"github.com/spf13/cobra"
)

var DBCmd = &cobra.Command{
	Use:   "db",
	Short: "Database administration commands",
}

func init() {
	DBCmd.AddCommand(initCmd)
}
