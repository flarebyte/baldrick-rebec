package server

import (
    "fmt"
    "os"
    "syscall"

    srv "github.com/flarebyte/baldrick-rebec/internal/server"
    "github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show current server state",
    RunE: func(cmd *cobra.Command, args []string) error {
        pidPath := srv.DefaultPIDPath()
        pid, err := srv.ReadPID(pidPath)
        if err != nil {
            fmt.Fprintln(os.Stderr, "server: not running (no pid)")
            return nil
        }
        proc, err := os.FindProcess(pid)
        if err != nil {
            fmt.Fprintf(os.Stderr, "server: stale pid %d\n", pid)
            return nil
        }
        // Signal 0 check (Unix); on other OSes, just report pid
        if err := proc.Signal(syscall.Signal(0)); err != nil {
            fmt.Fprintf(os.Stderr, "server: not running (pid=%d not alive)\n", pid)
            return nil
        }
        fmt.Fprintf(os.Stderr, "server: running (pid=%d)\n", pid)
        return nil
    },
}

