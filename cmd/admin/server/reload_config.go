package server

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	srv "github.com/flarebyte/baldrick-rebec/internal/server"
	"github.com/spf13/cobra"
)

var reloadConfigCmd = &cobra.Command{
	Use:   "reload-config",
	Short: "Reload config files without restart",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidPath := srv.DefaultPIDPath()
		pid, err := srv.ReadPID(pidPath)
		if err != nil {
			return err
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		if runtime.GOOS == "windows" {
			// No SIGHUP on Windows. For now we simply report unsupported.
			fmt.Fprintln(os.Stderr, "reload-config is not supported on Windows")
			return nil
		}
		if err := proc.Signal(syscall.SIGHUP); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "reload-config signal sent to pid=%d\n", pid)
		return nil
	},
}

func init() {
	ServerCmd.AddCommand(reloadConfigCmd)
}
