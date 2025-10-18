package server

import (
    "errors"
    "fmt"
    "net"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"

    "github.com/flarebyte/baldrick-rebec/internal/config"
    "github.com/flarebyte/baldrick-rebec/internal/paths"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
)

func DefaultPIDPath() string {
    h := paths.Home()
    _ = os.MkdirAll(h, 0o755)
    return filepath.Join(h, "server.pid")
}

func RunForeground(addr, pidPath string) error {
    if err := writePID(pidPath); err != nil {
        return err
    }
    defer removePID(pidPath)

    lis, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("listen: %w", err)
    }
    s := grpc.NewServer()
    reflection.Register(s)

    // Graceful shutdown on SIGTERM/SIGINT and config reload on SIGHUP
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
    go func() {
        for {
            sig := <-sigCh
            switch sig {
            case syscall.SIGTERM, syscall.SIGINT:
                s.GracefulStop()
                return
            case syscall.SIGHUP:
                // Reload config; dynamic settings (like ports) require restart; we just refresh values.
                if _, err := config.Load(); err != nil {
                    fmt.Fprintf(os.Stderr, "server: reload-config failed: %v\n", err)
                } else {
                    fmt.Fprintln(os.Stderr, "server: config reloaded")
                }
            }
        }
    }()

    if err := s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
        return err
    }
    return nil
}

func writePID(pidPath string) error {
    if _, err := os.Stat(pidPath); err == nil {
        // existing pid file
        return fmt.Errorf("pid file exists: %s", pidPath)
    }
    pid := os.Getpid()
    f, err := os.OpenFile(pidPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = fmt.Fprintf(f, "%d", pid)
    return err
}

func removePID(pidPath string) {
    _ = os.Remove(pidPath)
}

func ReadPID(pidPath string) (int, error) {
    b, err := os.ReadFile(pidPath)
    if err != nil {
        return 0, err
    }
    var pid int
    if _, err := fmt.Sscanf(string(b), "%d", &pid); err != nil {
        return 0, err
    }
    return pid, nil
}

// DetachAttr returns platform-specific attributes to detach a process.
func DetachAttr() *syscall.SysProcAttr {
    return &syscall.SysProcAttr{Setsid: true}
}
