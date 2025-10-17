package server

import (
    "fmt"
    "os"
    "syscall"

    srv "github.com/flarebyte/baldrick-rebec/internal/server"
    "github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
    Use:   "stop",
    Short: "Stop the running server gracefully",
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
        // Best effort graceful stop via SIGTERM (Unix). On Windows, Kill.
        if err := proc.Signal(syscall.SIGTERM); err != nil {
            // Fallback to Kill
            _ = proc.Kill()
        }
        fmt.Fprintf(os.Stderr, "stop signal sent to pid=%d\n", pid)
        return nil
    },
}

