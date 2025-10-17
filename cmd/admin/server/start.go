package server

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"

    srv "github.com/flarebyte/baldrick-rebec/internal/server"
    "github.com/spf13/cobra"
)

var (
    flagDetach bool
    flagAddr   string
)

var startCmd = &cobra.Command{
    Use:   "start",
    Short: "Start the local server",
    RunE: func(cmd *cobra.Command, args []string) error {
        exe, err := os.Executable()
        if err != nil {
            return err
        }

        pidPath := srv.DefaultPIDPath()
        if flagDetach {
            // Spawn a detached child running in foreground mode
            child := exec.Command(exe, "admin", "server", "start", "--addr", flagAddr, "--no-detach")
            // Best-effort: redirect output to a basic log file next to pid
            logPath := filepath.Join(filepath.Dir(pidPath), "server.log")
            _ = os.MkdirAll(filepath.Dir(pidPath), 0o755)
            lf, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
            if lf != nil {
                defer lf.Close()
                child.Stdout = lf
                child.Stderr = lf
            }
            if runtime.GOOS == "windows" {
                // No special detaching; rely on Go spawning separate process
            } else {
                child.SysProcAttr = srv.DetachAttr()
            }
            if err := child.Start(); err != nil {
                return err
            }
            fmt.Fprintf(os.Stderr, "server started in background (pid=%d)\n", child.Process.Pid)
            return nil
        }

        // Foreground mode
        return srv.RunForeground(flagAddr, pidPath)
    },
}

func init() {
    startCmd.Flags().BoolVar(&flagDetach, "detach", false, "Run in background")
    // Hidden internal flag to prevent loop when re-execing for detach
    startCmd.Flags().Bool("no-detach", false, "internal")
    _ = startCmd.Flags().MarkHidden("no-detach")
    startCmd.Flags().StringVar(&flagAddr, "addr", srv.DefaultAddr, "Listen address for gRPC server")
}

